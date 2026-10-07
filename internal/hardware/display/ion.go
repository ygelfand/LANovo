package display

import (
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/ygelfand/LANovo/internal/layout"
)

const (
	ionAlloc = 3<<30 | 20<<16 | 'I'<<8 | 0
	ionFree  = 3<<30 | 4<<16 | 'I'<<8 | 1
	ionShare = 3<<30 | 8<<16 | 'I'<<8 | 4
)

type ionBuffer struct {
	mem []byte
	fd  int
}

func ionAllocate(size int, heaps uint32) (*ionBuffer, error) {
	dev, err := os.OpenFile(layout.IonDevice, os.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("ion: %w", err)
	}
	defer dev.Close()

	req := make([]byte, 20)
	binary.LittleEndian.PutUint32(req[0:], uint32(size))
	binary.LittleEndian.PutUint32(req[4:], 4096)
	binary.LittleEndian.PutUint32(req[8:], heaps)
	if err := rawIoctl(dev.Fd(), ionAlloc, req); err != nil {
		return nil, fmt.Errorf("ion: allocating %d bytes: %w", size, err)
	}
	handle := req[16:20]

	share := make([]byte, 8)
	copy(share, handle)
	err = rawIoctl(dev.Fd(), ionShare, share)
	rawIoctl(dev.Fd(), ionFree, append([]byte(nil), handle...))
	if err != nil {
		return nil, fmt.Errorf("ion: sharing: %w", err)
	}
	fd := int(int32(binary.LittleEndian.Uint32(share[4:])))

	mem, err := syscall.Mmap(fd, 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("ion: mapping %d bytes: %w", size, err)
	}
	return &ionBuffer{mem: mem, fd: fd}, nil
}

func (b *ionBuffer) close() {
	syscall.Munmap(b.mem)
	syscall.Close(b.fd)
}

func rawIoctl(fd uintptr, req uintptr, arg []byte) error {
	if _, _, e := syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		req,
		uintptr(unsafe.Pointer(&arg[0])),
	); e != 0 {
		return e
	}
	return nil
}
