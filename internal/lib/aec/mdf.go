// Copyright (C) 2003-2008 Jean-Marc Valin
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

// Ported from speexdsp libspeexdsp/mdf.c, the float build.

const (
	mdfMinLeak      = .005
	mdfVar1Smooth   = .36
	mdfVar2Smooth   = .7225
	mdfVar1Update   = .5
	mdfVar2Update   = .25
	mdfVarBacktrack = 4
)

// MDF is speexdsp's multidelay block frequency-domain echo canceller.
type MDF struct {
	frame, n, m int
	adapting    bool

	tf  *fft.FFT
	buf []complex64

	cancelCount int
	adapted     bool
	saturated   int
	screwedUp   int

	specAverage, beta0, betaMax float32
	sumAdapt, leak              float32
	pey, pyy                    float32
	davg1, davg2, dvar1, dvar2  float32

	e, x, y, input, wtmp []float32
	xs                   [][]complex64
	yspec, espec, phi    []complex64
	w, fg                [][]complex64

	power, power1, rf, yf, xf, eh, yh []float32
	window, prop                      []float32

	lastY, residY []float32
	residSpec     []complex64

	preemph, notchRadius float32
	notch                [2]float32
	memX, memD, memE     float32

	out        []int16
	sumD, sumE float64
}

// NewMDF builds one for frame samples at a time, a power of two, reaching taps samples of echo tail.
func NewMDF(frame, taps, rate int) (*MDF, error) {
	if frame < 2 || frame&(frame-1) != 0 {
		return nil, fmt.Errorf("aec: frame must be a power of two, got %d", frame)
	}
	m := (taps + frame - 1) / frame
	if m < 2 {
		return nil, fmt.Errorf("aec: taps must reach at least two frames, got %d", taps)
	}

	n := 2 * frame
	s := &MDF{
		frame: frame, n: n, m: m, adapting: true,
		tf: fft.New(n), buf: make([]complex64, n),

		specAverage: float32(frame) / float32(rate),
		beta0:       2 * float32(frame) / float32(rate),
		betaMax:     .5 * float32(frame) / float32(rate),
		preemph:     .9,

		e: make([]float32, n), x: make([]float32, n), y: make([]float32, n),
		input: make([]float32, frame), wtmp: make([]float32, n),
		yspec: make([]complex64, frame+1), espec: make([]complex64, frame+1), phi: make([]complex64, frame+1),

		power: make([]float32, frame+1), power1: make([]float32, frame+1),
		rf: make([]float32, frame+1), yf: make([]float32, frame+1), xf: make([]float32, frame+1),
		eh: make([]float32, frame+1), yh: make([]float32, frame+1),
		window: make([]float32, n), prop: make([]float32, m),
		lastY: make([]float32, n), residY: make([]float32, n), residSpec: make([]complex64, frame+1),
	}
	switch {
	case rate < 12000:
		s.notchRadius = .9
	case rate < 24000:
		s.notchRadius = .982
	default:
		s.notchRadius = .992
	}

	s.xs = make([][]complex64, m+1)
	for i := range s.xs {
		s.xs[i] = make([]complex64, frame+1)
	}
	s.w, s.fg = make([][]complex64, m), make([][]complex64, m)
	for i := range m {
		s.w[i], s.fg[i] = make([]complex64, frame+1), make([]complex64, frame+1)
	}

	for i := range n {
		s.window[i] = float32(.5 - .5*math.Cos(2*math.Pi*float64(i)/float64(n)))
	}
	decay := float32(math.Exp(-2.4 / float64(m)))
	s.prop[0] = .7
	sum := s.prop[0]
	for i := 1; i < m; i++ {
		s.prop[i] = s.prop[i-1] * decay
		sum += s.prop[i]
	}
	for i := range s.prop {
		s.prop[i] = .8 * s.prop[i] / sum
	}

	s.Reset()
	return s, nil
}

// Reset forgets the room.
func (s *MDF) Reset() {
	s.cancelCount, s.screwedUp, s.saturated = 0, 0, 0
	for i := range s.m {
		clear(s.w[i])
		clear(s.fg[i])
	}
	for i := range s.xs {
		clear(s.xs[i])
	}
	for i := range s.power {
		s.power[i], s.power1[i], s.eh[i], s.yh[i] = 0, 1, 0, 0
	}
	clear(s.espec)
	clear(s.x)
	s.notch = [2]float32{}
	s.memD, s.memE, s.memX = 0, 0, 0
	s.adapted, s.sumAdapt = false, 0
	s.pey, s.pyy = 1, 1
	s.davg1, s.davg2, s.dvar1, s.dvar2 = 0, 0, 0, 0
	s.sumD, s.sumE = 0, 0
	clear(s.lastY)
}

// SetAdapting stops or resumes learning while still cancelling with what it has.
func (s *MDF) SetAdapting(on bool) { s.adapting = on }

// ERLE is how much quieter the output is than the microphone, in dB, over roughly the last half second.
func (s *MDF) ERLE() float64 {
	if s.sumE <= 0 {
		return 0
	}
	return 10 * math.Log10(s.sumD/s.sumE)
}

func (s *MDF) Frame() int { return s.frame }

func (s *MDF) Residual(dst []float32) {
	for i := range s.residY {
		s.residY[i] = s.window[i] * s.lastY[i]
	}
	s.forward(s.residSpec, s.residY)
	leak := float32(1)
	if s.leak <= .5 {
		leak = 2 * s.leak
	}
	for k := range min(len(dst), len(s.residSpec)) {
		c := s.residSpec[k]
		dst[k] = leak * (real(c)*real(c) + imag(c)*imag(c))
	}
}

// Process cancels the echo of ref from mic. Both are a whole number of frames.
func (s *MDF) Process(mic, ref []int16) ([]int16, error) {
	if len(mic) != len(ref) {
		return nil, ErrLength
	}
	if len(mic)%s.frame != 0 {
		return nil, fmt.Errorf("aec: %d samples is not a whole number of %d sample frames", len(mic), s.frame)
	}
	if cap(s.out) < len(mic) {
		s.out = make([]int16, len(mic))
	}
	s.out = s.out[:len(mic)]

	for at := 0; at < len(mic); at += s.frame {
		s.step(mic[at:at+s.frame], ref[at:at+s.frame], s.out[at:at+s.frame])
	}

	for i := range mic {
		d, e := float64(mic[i])/full, float64(s.out[i])/full
		s.sumD = s.sumD*(1-1.0/erleTau) + d*d
		s.sumE = s.sumE*(1-1.0/erleTau) + e*e
	}
	return s.out, nil
}

func (s *MDF) step(in, far []int16, out []int16) {
	f, n, m := s.frame, s.n, s.m
	s.cancelCount++
	ss := float32(.35) / float32(m)
	ss1 := 1 - ss

	s.dcNotch(in)
	for i := range f {
		v := s.input[i] - s.preemph*s.memD
		s.memD = s.input[i]
		s.input[i] = v
	}

	for i := range f {
		s.x[i] = s.x[i+f]
		v := float32(far[i])
		s.x[i+f] = v - s.preemph*s.memX
		s.memX = v
	}

	last := s.xs[m]
	copy(s.xs[1:], s.xs[:m])
	s.xs[0] = last
	s.forward(s.xs[0], s.x)

	sxx := inner(s.x[f:], s.x[f:])
	powerAccum(s.xs[0], s.xf)

	mulAccum(s.xs[:m], s.fg, s.yspec)
	s.inverse(s.e, s.yspec)
	for i := range f {
		s.e[i] = s.input[i] - s.e[i+f]
	}
	sff := inner(s.e[:f], s.e[:f])

	if s.adapted {
		s.adjustProp()
	}
	if s.saturated == 0 {
		if s.adapting {
			for j := m - 1; j >= 0; j-- {
				s.weightedConj(s.prop[j], s.xs[j+1])
				for i := range s.phi {
					s.w[j][i] += s.phi[i]
				}
			}
		}
	} else {
		s.saturated--
	}

	for j := range m {
		if j == 0 || s.cancelCount%(m-1) == j-1 {
			s.inverse(s.wtmp, s.w[j])
			clear(s.wtmp[f:])
			s.forward(s.w[j], s.wtmp)
		}
	}

	clear(s.rf)
	clear(s.yf)
	clear(s.xf)

	mulAccum(s.xs[:m], s.w, s.yspec)
	s.inverse(s.y, s.yspec)
	for i := range f {
		s.e[i] = s.e[i+f] - s.y[i+f]
	}
	dbf := 10 + inner(s.e[:f], s.e[:f])
	for i := range f {
		s.e[i] = s.input[i] - s.y[i+f]
	}
	see := inner(s.e[:f], s.e[:f])

	d := sff - see
	s.davg1 = .6*s.davg1 + .4*d
	s.davg2 = .85*s.davg2 + .15*d
	s.dvar1 = mdfVar1Smooth*s.dvar1 + .16*sff*dbf
	s.dvar2 = mdfVar2Smooth*s.dvar2 + .0225*sff*dbf

	switch {
	case d*abs32(d) > sff*dbf,
		s.davg1*abs32(s.davg1) > mdfVar1Update*s.dvar1,
		s.davg2*abs32(s.davg2) > mdfVar2Update*s.dvar2:
		s.davg1, s.davg2, s.dvar1, s.dvar2 = 0, 0, 0, 0
		for j := range m {
			copy(s.fg[j], s.w[j])
		}
		for i := range f {
			s.e[i+f] = s.window[i+f]*s.e[i+f] + s.window[i]*s.y[i+f]
		}
	case -d*abs32(d) > mdfVarBacktrack*sff*dbf,
		-s.davg1*abs32(s.davg1) > mdfVarBacktrack*s.dvar1,
		-s.davg2*abs32(s.davg2) > mdfVarBacktrack*s.dvar2:
		for j := range m {
			copy(s.w[j], s.fg[j])
		}
		for i := range f {
			s.y[i+f] = s.e[i+f]
		}
		for i := range f {
			s.e[i] = s.input[i] - s.y[i+f]
		}
		see = sff
		s.davg1, s.davg2, s.dvar1, s.dvar2 = 0, 0, 0, 0
	}

	for i := range f {
		v := s.input[i] - s.e[i+f] + s.preemph*s.memE
		if in[i] <= -32000 || in[i] >= 32000 {
			if s.saturated == 0 {
				s.saturated = 1
			}
		}
		out[i] = toInt16(v)
		s.memE = v
	}

	copy(s.lastY, s.lastY[f:])
	if s.adapted {
		for i := range f {
			s.lastY[f+i] = float32(in[i]) - float32(out[i])
		}
	}

	for i := range f {
		s.e[i+f] = s.e[i]
		s.e[i] = 0
	}
	sey := inner(s.e[f:], s.y[f:])
	syy := inner(s.y[f:], s.y[f:])
	sdd := inner(s.input, s.input)

	s.forward(s.espec, s.e)
	clear(s.y[:f])
	s.forward(s.yspec, s.y)
	powerAccum(s.espec, s.rf)
	powerAccum(s.yspec, s.yf)

	nf := float32(n)
	switch {
	case !(syy >= 0 && sxx >= 0 && see >= 0) || !(sff < nf*1e9 && syy < nf*1e9 && sxx < nf*1e9):
		s.screwedUp += 50
		clear(out)
	case sff > sdd+nf*10000:
		s.screwedUp++
	default:
		s.screwedUp = 0
	}
	if s.screwedUp >= 50 {
		s.Reset()
		return
	}

	see = max(see, nf*100)

	sxx += inner(s.x[f:], s.x[f:])
	powerAccum(s.xs[0], s.xf)

	for j := range s.power {
		s.power[j] = ss1*s.power[j] + 1 + ss*s.xf[j]
	}

	pey, pyy := float32(1), float32(1)
	for j := f; j >= 0; j-- {
		eh := s.rf[j] - s.eh[j]
		yh := s.yf[j] - s.yh[j]
		pey += eh * yh
		pyy += yh * yh
		s.eh[j] = (1-s.specAverage)*s.eh[j] + s.specAverage*s.rf[j]
		s.yh[j] = (1-s.specAverage)*s.yh[j] + s.specAverage*s.yf[j]
	}
	pyy = float32(math.Sqrt(float64(pyy)))
	pey /= pyy

	t := s.beta0 * syy
	if t > s.betaMax*see {
		t = s.betaMax * see
	}
	alpha := t / see
	s.pey = (1-alpha)*s.pey + alpha*pey
	s.pyy = (1-alpha)*s.pyy + alpha*pyy
	if s.pyy < 1 {
		s.pyy = 1
	}
	if s.pey < mdfMinLeak*s.pyy {
		s.pey = mdfMinLeak * s.pyy
	}
	if s.pey > s.pyy {
		s.pey = s.pyy
	}
	s.leak = s.pey / s.pyy

	rer := (.0001*sxx + 3*s.leak*syy) / see
	if b := sey * sey / (1 + see*syy); rer < b {
		rer = b
	}
	if rer > .5 {
		rer = .5
	}

	if !s.adapted && s.sumAdapt > float32(m) && s.leak*syy > .03*syy {
		s.adapted = true
	}

	if s.adapted {
		for i := range s.power1 {
			r := s.leak * s.yf[i]
			e := s.rf[i] + 1
			if r > .5*e {
				r = .5 * e
			}
			r = .7*r + .3*rer*e
			s.power1[i] = r / (e * (s.power[i] + 10))
		}
		return
	}

	var rate float32
	if sxx > nf*1000 {
		t := .25 * sxx
		if t > .25*see {
			t = .25 * see
		}
		rate = t / see
	}
	for i := range s.power1 {
		s.power1[i] = rate / (s.power[i] + 10)
	}
	s.sumAdapt += rate
}

func (s *MDF) dcNotch(in []int16) {
	r := s.notchRadius
	den2 := r*r + .7*(1-r)*(1-r)
	for i, v := range in {
		vin := float32(v)
		vout := s.notch[0] + vin
		s.notch[0] = s.notch[1] + 2*(-vin+r*vout)
		s.notch[1] = vin - den2*vout
		s.input[i] = min(max(r*vout, -32767), 32767)
	}
}

func (s *MDF) adjustProp() {
	maxSum, propSum := float32(1), float32(1)
	for i := range s.m {
		t := float32(1)
		for _, c := range s.w[i] {
			t += real(c)*real(c) + imag(c)*imag(c)
		}
		s.prop[i] = float32(math.Sqrt(float64(t)))
		maxSum = max(maxSum, s.prop[i])
	}
	for i := range s.m {
		s.prop[i] += .1 * maxSum
		propSum += s.prop[i]
	}
	for i := range s.m {
		s.prop[i] = .99 * s.prop[i] / propSum
	}
}

func (s *MDF) weightedConj(p float32, x []complex64) {
	for k := range s.phi {
		w := complex(p*s.power1[k], 0)
		s.phi[k] = w * conj(x[k]) * s.espec[k]
	}
}

func (s *MDF) forward(dst []complex64, src []float32) {
	scale := 1 / float32(s.n)
	for i, v := range src {
		s.buf[i] = complex(v, 0)
	}
	s.tf.Forward(s.buf)
	for k := range dst {
		dst[k] = s.buf[k] * complex(scale, 0)
	}
}

func (s *MDF) inverse(dst []float32, src []complex64) {
	f, n := s.frame, s.n
	for k := 0; k <= f; k++ {
		s.buf[k] = src[k]
	}
	for k := 1; k < f; k++ {
		s.buf[n-k] = conj(src[k])
	}
	s.buf[0] = complex(real(src[0]), 0)
	s.buf[f] = complex(real(src[f]), 0)
	s.tf.Inverse(s.buf)
	for i := range dst {
		dst[i] = real(s.buf[i]) * float32(n)
	}
}

func mulAccum(x, w [][]complex64, acc []complex64) {
	clear(acc)
	for j := range w {
		for k := range acc {
			acc[k] += x[j][k] * w[j][k]
		}
	}
}

func powerAccum(x []complex64, ps []float32) {
	for k, c := range x {
		ps[k] += real(c)*real(c) + imag(c)*imag(c)
	}
}

func inner(a, b []float32) float32 {
	var sum float32
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}

func conj(c complex64) complex64 { return complex(real(c), -imag(c)) }

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func toInt16(v float32) int16 {
	switch {
	case v < -32767.5:
		return -32768
	case v > 32766.5:
		return 32767
	}
	return int16(math.Floor(float64(.5 + v)))
}
