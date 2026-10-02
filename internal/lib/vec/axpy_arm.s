//go:build arm && !noasm

#include "textflag.h"

// func axpyVFP(dst, x *float32, n int, gain float32)
//
// dst[i] += gain*x[i], sixteen floats an iteration, in blocks of six, six and four.
//
// The target is a Cortex-A53, two-wide and in-order. Two things are being arranged. A block's loads
// all go before any of its arithmetic and all of its stores after, so a value has the rest of the
// block between being computed and being written — a multiply-accumulate has several cycles of
// latency and a store that asks for the result too early pays all of it. And six pairs rather than
// four, because twelve registers is what there is: the wider the block, the further apart those two
// ends can be.
//
// The pointers move by hand: VLDR and VSTR have no post-indexed form, and MOVF.P assembles without the
// writeback rather than refusing.
TEXT ·axpyVFP(SB), NOSPLIT, $0-16
	MOVW	dst+0(FP), R0
	MOVW	x+4(FP), R1
	MOVW	n+8(FP), R2
	MOVF	gain+12(FP), F0

sixteen:
	CMP	$16, R2
	BLT	four

	MOVF	0(R0), F4
	MOVF	0(R1), F5
	MOVF	4(R0), F6
	MOVF	4(R1), F7
	MOVF	8(R0), F8
	MOVF	8(R1), F9
	MOVF	12(R0), F10
	MOVF	12(R1), F11
	MOVF	16(R0), F12
	MOVF	16(R1), F13
	MOVF	20(R0), F14
	MOVF	20(R1), F15

	MULAF	F0, F5, F4
	MULAF	F0, F7, F6
	MULAF	F0, F9, F8
	MULAF	F0, F11, F10
	MULAF	F0, F13, F12
	MULAF	F0, F15, F14

	MOVF	F4, 0(R0)
	MOVF	F6, 4(R0)
	MOVF	F8, 8(R0)
	MOVF	F10, 12(R0)
	MOVF	F12, 16(R0)
	MOVF	F14, 20(R0)

	MOVF	24(R0), F4
	MOVF	24(R1), F5
	MOVF	28(R0), F6
	MOVF	28(R1), F7
	MOVF	32(R0), F8
	MOVF	32(R1), F9
	MOVF	36(R0), F10
	MOVF	36(R1), F11
	MOVF	40(R0), F12
	MOVF	40(R1), F13
	MOVF	44(R0), F14
	MOVF	44(R1), F15

	MULAF	F0, F5, F4
	MULAF	F0, F7, F6
	MULAF	F0, F9, F8
	MULAF	F0, F11, F10
	MULAF	F0, F13, F12
	MULAF	F0, F15, F14

	// In the gap rather than beside the branch: the integer pipe is idle through the multiplies.
	SUB	$16, R2

	MOVF	F4, 24(R0)
	MOVF	F6, 28(R0)
	MOVF	F8, 32(R0)
	MOVF	F10, 36(R0)
	MOVF	F12, 40(R0)
	MOVF	F14, 44(R0)

	MOVF	48(R0), F4
	MOVF	48(R1), F5
	MOVF	52(R0), F6
	MOVF	52(R1), F7
	MOVF	56(R0), F8
	MOVF	56(R1), F9
	MOVF	60(R0), F10
	MOVF	60(R1), F11

	MULAF	F0, F5, F4
	MULAF	F0, F7, F6
	MULAF	F0, F9, F8
	MULAF	F0, F11, F10

	MOVF	F4, 48(R0)
	MOVF	F6, 52(R0)
	MOVF	F8, 56(R0)
	MOVF	F10, 60(R0)

	ADD	$64, R0
	ADD	$64, R1
	B	sixteen

four:
	CMP	$4, R2
	BLT	one

	MOVF	0(R0), F4
	MOVF	0(R1), F5
	MOVF	4(R0), F6
	MOVF	4(R1), F7
	MOVF	8(R0), F8
	MOVF	8(R1), F9
	MOVF	12(R0), F10
	MOVF	12(R1), F11

	MULAF	F0, F5, F4
	MULAF	F0, F7, F6
	MULAF	F0, F9, F8
	MULAF	F0, F11, F10

	MOVF	F4, 0(R0)
	MOVF	F6, 4(R0)
	MOVF	F8, 8(R0)
	MOVF	F10, 12(R0)

	ADD	$16, R0
	ADD	$16, R1
	SUB	$4, R2
	B	four

one:
	CMP	$0, R2
	BEQ	done

	MOVF	0(R0), F4
	MOVF	0(R1), F5
	MULAF	F0, F5, F4
	MOVF	F4, 0(R0)
	ADD	$4, R0
	ADD	$4, R1
	SUB	$1, R2
	B	one

done:
	RET
