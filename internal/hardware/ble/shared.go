package ble

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ygelfand/libcountertop/pkg/bluetooth/hci"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/runtime/service"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
)

const Node = "/dev/stpbt"
const TTY = "/dev/ttyHS0"

const (
	patchFile = "btfwnpla.tlv"
	nvmFile   = "btnvnpla.bin"
)

// Qualcomm builds ship the Bluetooth patch on bt_firmware or in the modem image.
var firmwareDirs = []string{"/bt_firmware/image", "/firmware/image"}

func firmwareDir() string {
	for _, d := range firmwareDirs {
		if _, err := os.Stat(filepath.Join(d, patchFile)); err == nil {
			return d
		}
	}
	return firmwareDirs[0]
}

var Listen bool

func Power(on bool) (bool, error) {
	if board.Current().SoC == board.MediaTek {
		return false, nil
	}
	return hci.Power(on, "/sys/class/rfkill")
}
func BringUp() (*hci.Port, hci.Version, error) {
	if board.Current().SoC != board.MediaTek {
		return hci.BringUp(
			hci.BringUpOptions{
				UART:              TTY,
				FirmwareDirectory: firmwareDir(),
				PatchFile:         patchFile,
				NVMFile:           nvmFile,
				Power:             Power,
				ListenAfterReset:  Listen,
			},
		)
	}
	return hci.BringUp(hci.BringUpOptions{Node: Node})
}

var get = sync.OnceValue(
	func() *hci.Radio {
		return hci.NewRadio(hci.RadioOptions{
			BringUp: BringUp,
			Power:   Power,
			Quirks:  board.Current().Bluetooth,
		})
	},
)

func Get() *hci.Radio { return get() }
func init() {
	component.Register(
		sharedcomponent.Hardware,
		Get,
		sharedcomponent.Order(30),
		sharedcomponent.Supervise(service.Restart(2*time.Second, time.Minute)),
	)
}
