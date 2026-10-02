package sbc

// The synthesis filterbank's constant tables.
//
// Taken from FFmpeg's libavcodec/sbcdec_data.h, which descends from the bluez sbc library:
//
//	Copyright (C) 2017       Aurelien Jacobs <aurel@gnuage.org>
//	Copyright (C) 2008-2010  Nokia Corporation
//	Copyright (C) 2004-2010  Marcel Holtmann <marcel@holtmann.org>
//	Copyright (C) 2004-2005  Henryk Ploetz <henryk@ploetzli.ch>
//	Copyright (C) 2005-2006  Brad Midgley <bmidgley@xmission.com>
//
//	Licensed under the GNU Lesser General Public License, version 2.1 or later.
//
// Copied rather than computed. The prototype window is a normative table in the A2DP
// specification with no formula that reproduces it, and the matrices here are not the plain cosine
// either — they are folded to match the loop in synthesis.go, so deriving them would mean deriving
// the folding as well. A window that is close but not identical gives audio that is quietly wrong
// rather than obviously broken, which is not worth the risk to save a table.
//
// Held as the source writes them, unshifted, so they can be read against it line by line. The
// shifts the source applies with its SS4/SS8/SN4/SN8 macros are applied once in shifted().

// proto4m0 and proto4m1 are the 40-coefficient window for four subbands, split into the two halves
// the loop consumes in step.
var proto4m0 = shifted(12, []uint32{
	0x00000000, 0xffa6982f, 0xfba93848, 0x0456c7b8,
	0x005967d1, 0xfffb9ac7, 0xff589157, 0xf9c2a8d8,
	0x027c1434, 0x0019118b, 0xfff3c74c, 0xff137330,
	0xf81b8d70, 0x00ec1b8b, 0xfff0b71a, 0xffe99b00,
	0xfef84470, 0xf6fb4370, 0xffcdc351, 0xffe01dc7,
})

var proto4m1 = shifted(12, []uint32{
	0xffe090ce, 0xff2c0475, 0xf694f800, 0xff2c0475,
	0xffe090ce, 0xffe01dc7, 0xffcdc351, 0xf6fb4370,
	0xfef84470, 0xffe99b00, 0xfff0b71a, 0x00ec1b8b,
	0xf81b8d70, 0xff137330, 0xfff3c74c, 0x0019118b,
	0x027c1434, 0xf9c2a8d8, 0xff589157, 0xfffb9ac7,
})

// proto8m0 and proto8m1 are the same for eight subbands, 80 coefficients.
var proto8m0 = shifted(14, []uint32{
	0x00000000, 0xfe8d1970, 0xee979f00, 0x11686100,
	0x0172e690, 0xfff5bd1a, 0xfdf1c8d4, 0xeac182c0,
	0x0d9daee0, 0x00e530da, 0xffe9811d, 0xfd52986c,
	0xe7054ca0, 0x0a00d410, 0x006c1de4, 0xffdba705,
	0xfcbc98e8, 0xe3889d20, 0x06af2308, 0x000bb7db,
	0xffca00ed, 0xfc3fbb68, 0xe071bc00, 0x03bf7948,
	0xffc4e05c, 0xffb54b3b, 0xfbedadc0, 0xdde26200,
	0x0142291c, 0xff960e94, 0xff9f3e17, 0xfbd8f358,
	0xdbf79400, 0xff405e01, 0xff7d4914, 0xff8b1a31,
	0xfc1417b8, 0xdac7bb40, 0xfdbb828c, 0xff762170,
})

var proto8m1 = shifted(14, []uint32{
	0xff7c272c, 0xfcb02620, 0xda612700, 0xfcb02620,
	0xff7c272c, 0xff762170, 0xfdbb828c, 0xdac7bb40,
	0xfc1417b8, 0xff8b1a31, 0xff7d4914, 0xff405e01,
	0xdbf79400, 0xfbd8f358, 0xff9f3e17, 0xff960e94,
	0x0142291c, 0xdde26200, 0xfbedadc0, 0xffb54b3b,
	0xffc4e05c, 0x03bf7948, 0xe071bc00, 0xfc3fbb68,
	0xffca00ed, 0x000bb7db, 0x06af2308, 0xe3889d20,
	0xfcbc98e8, 0xffdba705, 0x006c1de4, 0x0a00d410,
	0xe7054ca0, 0xfd52986c, 0xffe9811d, 0x00e530da,
	0x0d9daee0, 0xeac182c0, 0xfdf1c8d4, 0xfff5bd1a,
})

// matrixShift is what the source's SN4 and SN8 macros come to: val >> (11 + 1 + extraBits). C shifts
// bind looser than the addition, which is worth saying because reading it the other way gives a
// different number.
const matrixShift = 11 + 1 + extraBits

// synMatrix4 is the 8x4 folded cosine matrix for four subbands.
var synMatrix4 = folded(matrixShift, 4, []uint32{
	0x05a82798, 0xfa57d868, 0xfa57d868, 0x05a82798,
	0x030fbc54, 0xf89be510, 0x07641af0, 0xfcf043ac,
	0x00000000, 0x00000000, 0x00000000, 0x00000000,
	0xfcf043ac, 0x07641af0, 0xf89be510, 0x030fbc54,
	0xfa57d868, 0x05a82798, 0x05a82798, 0xfa57d868,
	0xf89be510, 0xfcf043ac, 0x030fbc54, 0x07641af0,
	0xf8000000, 0xf8000000, 0xf8000000, 0xf8000000,
	0xf89be510, 0xfcf043ac, 0x030fbc54, 0x07641af0,
})

// synMatrix8 is the 16x8 for eight subbands.
var synMatrix8 = folded(matrixShift, 8, []uint32{
	0x05a82798, 0xfa57d868, 0xfa57d868, 0x05a82798, 0x05a82798, 0xfa57d868, 0xfa57d868, 0x05a82798,
	0x0471ced0, 0xf8275a10, 0x018f8b84, 0x06a6d988, 0xf9592678, 0xfe70747c, 0x07d8a5f0, 0xfb8e3130,
	0x030fbc54, 0xf89be510, 0x07641af0, 0xfcf043ac, 0xfcf043ac, 0x07641af0, 0xf89be510, 0x030fbc54,
	0x018f8b84, 0xfb8e3130, 0x06a6d988, 0xf8275a10, 0x07d8a5f0, 0xf9592678, 0x0471ced0, 0xfe70747c,
	0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000,
	0xfe70747c, 0x0471ced0, 0xf9592678, 0x07d8a5f0, 0xf8275a10, 0x06a6d988, 0xfb8e3130, 0x018f8b84,
	0xfcf043ac, 0x07641af0, 0xf89be510, 0x030fbc54, 0x030fbc54, 0xf89be510, 0x07641af0, 0xfcf043ac,
	0xfb8e3130, 0x07d8a5f0, 0xfe70747c, 0xf9592678, 0x06a6d988, 0x018f8b84, 0xf8275a10, 0x0471ced0,
	0xfa57d868, 0x05a82798, 0x05a82798, 0xfa57d868, 0xfa57d868, 0x05a82798, 0x05a82798, 0xfa57d868,
	0xf9592678, 0x018f8b84, 0x07d8a5f0, 0x0471ced0, 0xfb8e3130, 0xf8275a10, 0xfe70747c, 0x06a6d988,
	0xf89be510, 0xfcf043ac, 0x030fbc54, 0x07641af0, 0x07641af0, 0x030fbc54, 0xfcf043ac, 0xf89be510,
	0xf8275a10, 0xf9592678, 0xfb8e3130, 0xfe70747c, 0x018f8b84, 0x0471ced0, 0x06a6d988, 0x07d8a5f0,
	0xf8000000, 0xf8000000, 0xf8000000, 0xf8000000, 0xf8000000, 0xf8000000, 0xf8000000, 0xf8000000,
	0xf8275a10, 0xf9592678, 0xfb8e3130, 0xfe70747c, 0x018f8b84, 0x0471ced0, 0x06a6d988, 0x07d8a5f0,
	0xf89be510, 0xfcf043ac, 0x030fbc54, 0x07641af0, 0x07641af0, 0x030fbc54, 0xfcf043ac, 0xf89be510,
	0xf9592678, 0x018f8b84, 0x07d8a5f0, 0x0471ced0, 0xfb8e3130, 0xf8275a10, 0xfe70747c, 0x06a6d988,
})

// shifted reads the constants as signed and applies the source's scaling, once.
func shifted(by uint, raw []uint32) []int32 {
	out := make([]int32, len(raw))
	for i, v := range raw {
		out[i] = int32(v) >> by
	}
	return out
}

// folded is the same for a table written flat and used as rows.
func folded(by uint, wide int, raw []uint32) [][]int32 {
	flat := shifted(by, raw)

	out := make([][]int32, len(raw)/wide)
	for i := range out {
		out[i] = flat[i*wide : (i+1)*wide]
	}
	return out
}
