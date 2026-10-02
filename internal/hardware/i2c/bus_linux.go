package i2c

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

// The driver's ioctls and the one flag that matters.
const (
	iocRDWR = 0x0707

	// mRead is I2C_M_RD: this leg reads rather than writes.
	mRead = 0x0001
)

// msg is struct i2c_msg. The pointer sizes itself per architecture, which is what makes this 12
// bytes on the device and 16 on a host.
type msg struct {
	addr  uint16
	flags uint16
	len   uint16
	buf   *byte
}

// rdwr is struct i2c_rdwr_ioctl_data.
type rdwr struct {
	msgs  *msg
	nmsgs uint32
}

// bus is one /dev/i2c-N.
type bus struct{ f *os.File }

// Open is the bus by its kernel number.
func Open(n int) (Bus, error) { return OpenPath(fmt.Sprintf("/dev/i2c-%d", n)) }

// OpenPath is the bus at a path.
func OpenPath(path string) (Bus, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("i2c: %s: %w", path, err)
	}
	return &bus{f: f}, nil
}

func (b *bus) Close() error { return b.f.Close() }

// Transfer sends every message as one exchange, so a register read keeps the bus between writing
// the register and reading the answer.
func (b *bus) Transfer(msgs ...Msg) error {
	if len(msgs) == 0 {
		return nil
	}

	out := make([]msg, len(msgs))
	for i, m := range msgs {
		if len(m.Buf) == 0 {
			return fmt.Errorf("i2c: message %d carries no bytes", i)
		}

		out[i] = msg{addr: m.Addr, len: uint16(len(m.Buf)), buf: &m.Buf[0]}
		if m.Read {
			out[i].flags = mRead
		}
	}

	data := rdwr{msgs: &out[0], nmsgs: uint32(len(out))}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, b.f.Fd(), iocRDWR, uintptr(unsafe.Pointer(&data)))

	// The kernel wrote through the pointers in out, which Go's collector does not know about.
	runtime.KeepAlive(msgs)
	runtime.KeepAlive(out)

	if errno != 0 {
		return fmt.Errorf("i2c: transfer of %d message(s): %w", len(msgs), errno)
	}
	return nil
}
