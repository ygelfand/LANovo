// Package dex reads constant tables out of Dalvik bytecode: the short[][] a class's static
// initializer builds from array literals.
package dex

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// File is a parsed classes.dex.
type File struct {
	b []byte

	strings, types, fields, methods, classes []byte
}

// Parse checks the header and finds the index tables.
func Parse(b []byte) (*File, error) {
	if len(b) < 0x70 || string(b[:4]) != "dex\n" {
		return nil, errors.New("dex: not a dex file")
	}
	f := &File{b: b}
	var err error
	table := func(at int, width int) []byte {
		n, off := int(u32(b, at)), int(u32(b, at+4))
		if err != nil {
			return nil
		}
		if off < 0 || n < 0 || off+n*width > len(b) {
			err = fmt.Errorf("dex: table at %#x runs past the file", at)
			return nil
		}
		return b[off : off+n*width]
	}
	f.strings = table(0x38, 4)
	f.types = table(0x40, 4)
	f.fields = table(0x50, 8)
	f.methods = table(0x58, 8)
	f.classes = table(0x60, 32)
	return f, err
}

func u16(b []byte, at int) uint16 { return binary.LittleEndian.Uint16(b[at:]) }
func u32(b []byte, at int) uint32 { return binary.LittleEndian.Uint32(b[at:]) }

func (f *File) inBounds(at, n int) bool { return at >= 0 && n >= 0 && at+n <= len(f.b) }

func (f *File) uleb(at int) (uint32, int, error) {
	var v uint32
	for i := 0; i < 5; i++ {
		if !f.inBounds(at+i, 1) {
			return 0, 0, errors.New("dex: truncated uleb128")
		}
		c := f.b[at+i]
		v |= uint32(c&0x7f) << (7 * i)
		if c&0x80 == 0 {
			return v, at + i + 1, nil
		}
	}
	return 0, 0, errors.New("dex: uleb128 too long")
}

// String is the string at an index, with MUTF-8 read as UTF-8.
func (f *File) String(i uint32) (string, error) {
	if int(i)*4+4 > len(f.strings) {
		return "", fmt.Errorf("dex: no string %d", i)
	}
	at := int(u32(f.strings, int(i)*4))
	_, at, err := f.uleb(at)
	if err != nil {
		return "", err
	}
	end := at
	for f.inBounds(end, 1) && f.b[end] != 0 {
		end++
	}
	if !f.inBounds(end, 1) {
		return "", errors.New("dex: unterminated string")
	}
	return string(f.b[at:end]), nil
}

func (f *File) typeName(i uint32) (string, error) {
	if int(i)*4+4 > len(f.types) {
		return "", fmt.Errorf("dex: no type %d", i)
	}
	return f.String(u32(f.types, int(i)*4))
}

func (f *File) fieldName(i uint32) (string, error) {
	if int(i)*8+8 > len(f.fields) {
		return "", fmt.Errorf("dex: no field %d", i)
	}
	return f.String(u32(f.fields, int(i)*8+4))
}

func (f *File) methodName(i uint32) (string, error) {
	if int(i)*8+8 > len(f.methods) {
		return "", fmt.Errorf("dex: no method %d", i)
	}
	return f.String(u32(f.methods, int(i)*8+4))
}

// ErrNoClass means the class is not defined in this file.
var ErrNoClass = errors.New("dex: class not defined here")

// Tables runs the static initializer of the class with this descriptor (Lpkg/Name;) and returns
// the short[][] static fields it builds, by field name.
func (f *File) Tables(class string) (map[string][][]uint16, error) {
	for c := 0; c+32 <= len(f.classes); c += 32 {
		name, err := f.typeName(u32(f.classes, c))
		if err != nil {
			return nil, err
		}
		if name != class {
			continue
		}
		insns, err := f.clinit(int(u32(f.classes, c+24)))
		if err != nil {
			return nil, err
		}
		return run(insns, f.fieldName, f.typeName)
	}
	return nil, ErrNoClass
}

// clinit is the bytecode of <clinit>, from the class_data_item at off.
func (f *File) clinit(off int) ([]uint16, error) {
	if off == 0 {
		return nil, errors.New("dex: class has no data")
	}
	var counts [4]uint32
	at := off
	for i := range counts {
		v, next, err := f.uleb(at)
		if err != nil {
			return nil, err
		}
		counts[i], at = v, next
	}
	for i := uint32(0); i < 2*(counts[0]+counts[1]); i++ {
		_, next, err := f.uleb(at)
		if err != nil {
			return nil, err
		}
		at = next
	}
	var method uint32
	for i := uint32(0); i < counts[2]; i++ {
		var d, code uint32
		var err error
		if d, at, err = f.uleb(at); err != nil {
			return nil, err
		}
		if _, at, err = f.uleb(at); err != nil {
			return nil, err
		}
		if code, at, err = f.uleb(at); err != nil {
			return nil, err
		}
		method += d
		name, err := f.methodName(method)
		if err != nil {
			return nil, err
		}
		if name != "<clinit>" || code == 0 {
			continue
		}
		c := int(code)
		if !f.inBounds(c, 16) {
			return nil, errors.New("dex: code item past the file")
		}
		n := int(u32(f.b, c+12))
		if !f.inBounds(c+16, 2*n) {
			return nil, errors.New("dex: instructions past the file")
		}
		insns := make([]uint16, n)
		for k := range insns {
			insns[k] = u16(f.b, c+16+2*k)
		}
		return insns, nil
	}
	return nil, errors.New("dex: class has no static initializer")
}

// run interprets the instructions an array-literal initializer is made of, and nothing else.
func run(insns []uint16, field, typ func(uint32) (string, error)) (map[string][][]uint16, error) {
	ints := map[int]int32{}
	rows := map[int][]uint16{}
	outer := map[int][][]uint16{}
	out := map[string][][]uint16{}

	for pc := 0; pc < len(insns); {
		op := insns[pc] & 0xff
		hi := int(insns[pc] >> 8)
		need := map[uint16]int{0x12: 1, 0x13: 2, 0x23: 2, 0x26: 3, 0x4d: 2, 0x69: 2, 0x0e: 1}[op]
		if need == 0 {
			return nil, fmt.Errorf("dex: instruction %#02x at %d is not an array literal", op, pc)
		}
		if pc+need > len(insns) {
			return nil, errors.New("dex: instruction runs past the method")
		}
		switch op {
		case 0x0e:
			return out, nil
		case 0x12:
			ints[hi&0xf] = int32(int8(byte(hi)) >> 4)
			delete(rows, hi&0xf)
		case 0x13:
			ints[hi] = int32(int16(insns[pc+1]))
			delete(rows, hi)
		case 0x23:
			dst, size := hi&0xf, int(ints[hi>>4])
			if size < 0 || size > 1<<16 {
				return nil, fmt.Errorf("dex: array of %d at %d", size, pc)
			}
			kind, err := typ(uint32(insns[pc+1]))
			if err != nil {
				return nil, err
			}
			delete(rows, dst)
			delete(outer, dst)
			switch kind {
			case "[S":
				rows[dst] = make([]uint16, size)
			case "[[S":
				outer[dst] = make([][]uint16, size)
			default:
				return nil, fmt.Errorf("dex: new-array of %s at %d", kind, pc)
			}
		case 0x26:
			rel := int(int32(uint32(insns[pc+1]) | uint32(insns[pc+2])<<16))
			p := pc + rel
			if p < 0 || p+4 > len(insns) || insns[p] != 0x0300 {
				return nil, fmt.Errorf("dex: no array payload for %d", pc)
			}
			width, n := int(insns[p+1]), int(uint32(insns[p+2])|uint32(insns[p+3])<<16)
			if width != 2 || p+4+n > len(insns) {
				return nil, fmt.Errorf("dex: payload at %d is not shorts", p)
			}
			rows[hi] = append([]uint16(nil), insns[p+4:p+4+n]...)
		case 0x4d:
			val, arr, idx := hi, int(insns[pc+1]&0xff), int(insns[pc+1]>>8)
			o, ok := outer[arr]
			i := int(ints[idx])
			if !ok || i < 0 || i >= len(o) {
				return nil, fmt.Errorf("dex: aput-object at %d misses its array", pc)
			}
			o[i] = rows[val]
		case 0x69:
			name, err := field(uint32(insns[pc+1]))
			if err != nil {
				return nil, err
			}
			if o, ok := outer[hi]; ok {
				out[name] = o
			}
		}
		pc += need
	}
	return nil, errors.New("dex: static initializer never returns")
}
