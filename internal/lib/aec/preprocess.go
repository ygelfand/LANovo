// Copyright (C) 2003 Epic Games (written by Jean-Marc Valin)
// Copyright (C) 2004-2006 Epic Games
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
// 1. Redistributions of source code must retain the above copyright notice,
// this list of conditions and the following disclaimer.
//
// 2. Redistributions in binary form must reproduce the above copyright
// notice, this list of conditions and the following disclaimer in the
// documentation and/or other materials provided with the distribution.
//
// 3. The name of the author may not be used to endorse or promote products
// derived from this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE AUTHOR ``AS IS'' AND ANY EXPRESS OR
// IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES
// OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY DIRECT,
// INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
// (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
// SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
// HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT,
// STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN
// ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package aec

import (
	"fmt"
	"math"

	"github.com/ygelfand/LANovo/internal/lib/fft"
)

// Ported from speexdsp libspeexdsp/preprocess.c and filterbank.c, the float build, without AGC, VAD or dereverb.

const (
	ppBands              = 24
	ppNoiseSuppress      = -15
	ppEchoSuppress       = -40
	ppEchoSuppressActive = -15
)

type Preprocessor struct {
	frame, n, m int
	echo        *MDF

	NoiseSuppress, EchoSuppress, EchoSuppressActive int

	bank *filterbank
	tf   *fft.FFT
	buf  []complex64

	window, frameBuf         []float32
	spec                     []complex64
	ps, noise, echoNoise     []float32
	residual, oldPS          []float32
	prior, post, gain, gain2 []float32
	gainFloor, zeta          []float32
	s, smin, stmp            []float32
	updateProb               []bool
	inbuf, outbuf            []float32

	nbAdapt, minCount int
}

func NewPreprocessor(frame, rate int, echo *MDF) (*Preprocessor, error) {
	if frame <= 0 || frame&(frame-1) != 0 {
		return nil, fmt.Errorf("aec: preprocessor frame %d is not a power of two", frame)
	}
	if echo != nil && echo.Frame() != frame {
		return nil, fmt.Errorf("aec: preprocessor frame %d does not match the canceller's %d", frame, echo.Frame())
	}
	n, m := frame, ppBands
	p := &Preprocessor{
		frame: frame, n: n, m: m, echo: echo,
		NoiseSuppress: ppNoiseSuppress, EchoSuppress: ppEchoSuppress, EchoSuppressActive: ppEchoSuppressActive,
		bank: newFilterbank(m, rate, n),
		tf:   fft.New(2 * n), buf: make([]complex64, 2*n),
		window: make([]float32, 2*n), frameBuf: make([]float32, 2*n), spec: make([]complex64, n+1),
		ps: make([]float32, n+m), noise: make([]float32, n+m), echoNoise: make([]float32, n+m),
		residual: make([]float32, n+1), oldPS: make([]float32, n+m),
		prior: make([]float32, n+m), post: make([]float32, n+m), gain: make([]float32, n+m), gain2: make([]float32, n+m),
		gainFloor: make([]float32, n+m), zeta: make([]float32, n+m),
		s: make([]float32, n), smin: make([]float32, n), stmp: make([]float32, n), updateProb: make([]bool, n),
		inbuf: make([]float32, n), outbuf: make([]float32, n),
	}
	conjWindow(p.window)
	for i := range n + m {
		p.noise[i], p.oldPS[i], p.gain[i], p.post[i], p.prior[i] = 1, 1, 1, 1, 1
	}
	for i := range p.updateProb {
		p.updateProb[i] = true
	}
	return p, nil
}

func conjWindow(w []float32) {
	l := len(w)
	for i := range w {
		x := 4 * float64(i) / float64(l)
		inv := false
		switch {
		case x < 1:
		case x < 2:
			x, inv = 2-x, true
		case x < 3:
			x, inv = x-2, true
		default:
			x = 4 - x
		}
		x *= 1.271903
		tmp := .5 - .5*math.Cos(.5*math.Pi*x)
		tmp *= tmp
		if inv {
			tmp = 1 - tmp
		}
		w[i] = float32(math.Sqrt(tmp))
	}
}

func (p *Preprocessor) SetEcho(echo *MDF) { p.echo = echo }

func (p *Preprocessor) Run(x []int16) error {
	if len(x)%p.frame != 0 {
		return fmt.Errorf("aec: %d samples is not a whole number of %d sample frames", len(x), p.frame)
	}
	for at := 0; at < len(x); at += p.frame {
		p.step(x[at : at+p.frame])
	}
	return nil
}

func (p *Preprocessor) step(x []int16) {
	n, m := p.n, p.m
	p.nbAdapt = min(p.nbAdapt+1, 20000)
	p.minCount++
	beta := max(.03, 1/float32(p.nbAdapt))
	beta1 := 1 - beta

	if p.echo != nil {
		p.echo.Residual(p.residual)
		if r := p.residual[0]; !(r >= 0 && r < float32(n)*1e9) {
			clear(p.residual)
		}
		for i := range n {
			p.echoNoise[i] = max(.6*p.echoNoise[i], p.residual[i])
		}
		p.bank.bank(p.echoNoise[:n], p.echoNoise[n:])
	} else {
		clear(p.echoNoise)
	}

	p.analysis(x)
	p.updateNoiseProb()

	for i := range n {
		if !p.updateProb[i] || p.ps[i] < p.noise[i] {
			p.noise[i] = max(0, beta1*p.noise[i]+beta*p.ps[i])
		}
	}
	p.bank.bank(p.noise[:n], p.noise[n:])

	if p.nbAdapt == 1 {
		copy(p.oldPS, p.ps)
	}

	for i := range n + m {
		tot := 1 + p.noise[i] + p.echoNoise[i]
		p.post[i] = min(p.ps[i]/tot-1, 100)
		r := p.oldPS[i] / (p.oldPS[i] + tot)
		gamma := .1 + .89*r*r
		p.prior[i] = min(gamma*max(0, p.post[i])+(1-gamma)*p.oldPS[i]/tot, 100)
	}

	p.zeta[0] = .7*p.zeta[0] + .3*p.prior[0]
	for i := 1; i < n-1; i++ {
		p.zeta[i] = .7*p.zeta[i] + .15*p.prior[i] + .075*p.prior[i-1] + .075*p.prior[i+1]
	}
	for i := n - 1; i < n+m; i++ {
		p.zeta[i] = .7*p.zeta[i] + .3*p.prior[i]
	}

	var zframe float32
	for i := n; i < n+m; i++ {
		zframe += p.zeta[i]
	}
	pframe := .1 + .899*qcurve(zframe/float32(m))

	effective := (1-pframe)*float32(p.EchoSuppress) + pframe*float32(p.EchoSuppressActive)
	computeGainFloor(p.NoiseSuppress, effective, p.noise[n:], p.echoNoise[n:], p.gainFloor[n:])

	for i := n; i < n+m; i++ {
		ratio := p.prior[i] / (p.prior[i] + 1)
		theta := ratio * (1 + p.post[i])
		p.gain[i] = min(1, ratio*hypergeomGain(theta))
		p.oldPS[i] = .2*p.oldPS[i] + .8*p.gain[i]*p.gain[i]*p.ps[i]
		p1 := .199 + .8*qcurve(p.zeta[i])
		q := 1 - pframe*p1
		p.gain2[i] = 1 / (1 + (q/(1-q))*(1+p.prior[i])*float32(math.Exp(float64(-theta))))
	}

	p.bank.psd(p.gain2[n:], p.gain2[:n])
	p.bank.psd(p.gain[n:], p.gain[:n])
	p.bank.psd(p.gainFloor[n:], p.gainFloor[:n])

	for i := range n {
		ratio := p.prior[i] / (p.prior[i] + 1)
		theta := ratio * (1 + p.post[i])
		g := min(1, ratio*hypergeomGain(theta))
		prob := p.gain2[i]
		if .333*g > p.gain[i] {
			g = 3 * p.gain[i]
		}
		p.gain[i] = g
		p.oldPS[i] = .2*p.oldPS[i] + .8*p.gain[i]*p.gain[i]*p.ps[i]
		if p.gain[i] < p.gainFloor[i] {
			p.gain[i] = p.gainFloor[i]
		}
		tmp := prob*sqrt32(p.gain[i]) + (1-prob)*sqrt32(p.gainFloor[i])
		p.gain2[i] = tmp * tmp
	}

	for i := range n {
		p.spec[i] *= complex(p.gain2[i], 0)
	}
	p.spec[n] *= complex(p.gain2[n-1], 0)

	p.synthesis(x)
}

func (p *Preprocessor) analysis(x []int16) {
	n := p.n
	copy(p.frameBuf, p.inbuf)
	for i, v := range x {
		p.frameBuf[n+i] = float32(v)
		p.inbuf[i] = float32(v)
	}
	for i := range p.frameBuf {
		p.buf[i] = complex(p.frameBuf[i]*p.window[i], 0)
	}
	p.tf.Forward(p.buf)
	scale := complex(1/float32(2*n), 0)
	for k := 0; k <= n; k++ {
		p.spec[k] = p.buf[k] * scale
	}
	for i := range n {
		c := p.spec[i]
		p.ps[i] = real(c)*real(c) + imag(c)*imag(c)
	}
	p.ps[0] = real(p.spec[0]) * real(p.spec[0])
	p.bank.bank(p.ps[:n], p.ps[n:])
}

func (p *Preprocessor) synthesis(x []int16) {
	n := p.n
	p.buf[0] = complex(real(p.spec[0]), 0)
	for k := 1; k < n; k++ {
		p.buf[k] = p.spec[k]
		p.buf[2*n-k] = conj(p.spec[k])
	}
	p.buf[n] = complex(real(p.spec[n]), 0)
	p.tf.Inverse(p.buf)
	for i := range p.frameBuf {
		p.frameBuf[i] = real(p.buf[i]) * float32(2*n) * p.window[i]
	}
	for i := range n {
		x[i] = word2int(p.outbuf[i] + p.frameBuf[i])
		p.outbuf[i] = p.frameBuf[n+i]
	}
}

func (p *Preprocessor) updateNoiseProb() {
	n := p.n
	for i := 1; i < n-1; i++ {
		p.s[i] = .8*p.s[i] + .05*p.ps[i-1] + .1*p.ps[i] + .05*p.ps[i+1]
	}
	p.s[0] = .8*p.s[0] + .2*p.ps[0]
	p.s[n-1] = .8*p.s[n-1] + .2*p.ps[n-1]

	if p.nbAdapt == 1 {
		clear(p.smin)
		clear(p.stmp)
	}

	var span int
	switch {
	case p.nbAdapt < 100:
		span = 15
	case p.nbAdapt < 1000:
		span = 50
	case p.nbAdapt < 10000:
		span = 150
	default:
		span = 300
	}
	if p.minCount > span {
		p.minCount = 0
		for i := range n {
			p.smin[i] = min(p.stmp[i], p.s[i])
			p.stmp[i] = p.s[i]
		}
	} else {
		for i := range n {
			p.smin[i] = min(p.smin[i], p.s[i])
			p.stmp[i] = min(p.stmp[i], p.s[i])
		}
	}
	for i := range n {
		p.updateProb[i] = .4*p.s[i] > p.smin[i]
	}
}

var hypergeomTable = [21]float32{
	0.82157, 1.02017, 1.20461, 1.37534, 1.53363, 1.68092, 1.81865,
	1.94811, 2.07038, 2.18638, 2.29688, 2.40255, 2.50391, 2.60144,
	2.69551, 2.78647, 2.87458, 2.96015, 3.04333, 3.12431, 3.20326,
}

func hypergeomGain(x float32) float32 {
	integer := float32(math.Floor(float64(2 * x)))
	ind := int(integer)
	if ind < 0 {
		return 1
	}
	if ind > 19 {
		return 1 + .1296/x
	}
	frac := 2*x - integer
	return ((1-frac)*hypergeomTable[ind] + frac*hypergeomTable[ind+1]) / sqrt32(x+.0001)
}

func qcurve(x float32) float32 { return 1 / (1 + .15/x) }

func computeGainFloor(noiseSuppress int, echoSuppress float32, noise, echo, floor []float32) {
	nf := float32(math.Exp(.2302585 * float64(noiseSuppress)))
	ef := float32(math.Exp(.2302585 * float64(echoSuppress)))
	for i := range floor {
		floor[i] = sqrt32(nf*noise[i]+ef*echo[i]) / sqrt32(1+noise[i]+echo[i])
	}
}

func sqrt32(v float32) float32 { return float32(math.Sqrt(float64(v))) }

func word2int(v float32) int16 {
	switch {
	case v < -32767.5:
		return -32768
	case v > 32766.5:
		return 32767
	}
	return int16(math.Floor(float64(.5 + v)))
}

type filterbank struct {
	left, right      []int
	filterL, filterR []float32
	banks            int
}

func toBark(f float64) float64 {
	return 13.1*math.Atan(.00074*f) + 2.24*math.Atan(f*f*1.85e-8) + 1e-4*f
}

func newFilterbank(banks, rate, n int) *filterbank {
	b := &filterbank{
		left: make([]int, n), right: make([]int, n),
		filterL: make([]float32, n), filterR: make([]float32, n),
		banks: banks,
	}
	df := float64(rate) / float64(2*n)
	maxMel := toBark(float64(rate) / 2)
	interval := maxMel / float64(banks-1)
	for i := range n {
		mel := toBark(float64(i) * df)
		if mel > maxMel {
			break
		}
		id1 := int(math.Floor(mel / interval))
		var val float64
		if id1 > banks-2 {
			id1, val = banks-2, 1
		} else {
			val = (mel - float64(id1)*interval) / interval
		}
		b.left[i], b.filterL[i] = id1, float32(1-val)
		b.right[i], b.filterR[i] = id1+1, float32(val)
	}
	return b
}

func (b *filterbank) bank(ps, mel []float32) {
	clear(mel[:b.banks])
	for i := range ps {
		mel[b.left[i]] += b.filterL[i] * ps[i]
		mel[b.right[i]] += b.filterR[i] * ps[i]
	}
}

func (b *filterbank) psd(mel, ps []float32) {
	for i := range ps {
		ps[i] = mel[b.left[i]]*b.filterL[i] + mel[b.right[i]]*b.filterR[i]
	}
}
