package ble

import (
	"sync"
	"time"

	sharedhci "github.com/ygelfand/libcountertop/pkg/bluetooth/hci"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/service"
)

type Advertisement = sharedhci.Advertisement
type Blob = sharedhci.Blob
type BringUpOptions = sharedhci.BringUpOptions
type Classic = sharedhci.Classic
type Features = sharedhci.Features
type Handle = sharedhci.Handle
type Local = sharedhci.Local
type Patch = sharedhci.Patch
type Port = sharedhci.Port
type Radio = sharedhci.Radio
type RadioOptions = sharedhci.RadioOptions
type Sends = sharedhci.Sends
type Session = sharedhci.Session
type Version = sharedhci.Version

var Ask = sharedhci.Ask
var Fast = sharedhci.Fast
var Open = sharedhci.Open
var OpenNode = sharedhci.OpenNode
var ReadBlob = sharedhci.ReadBlob
var ReadFeatures = sharedhci.ReadFeatures
var ReadLocal = sharedhci.ReadLocal
var Reader = sharedhci.Reader
var Reset = sharedhci.Reset
var Scan = sharedhci.Scan
var Send = sharedhci.Send
var Serve = sharedhci.Serve
var SpeakerClass = sharedhci.SpeakerClass

const Node = "/dev/stpbt"
const TTY = "/dev/ttyHS0"

var Listen bool

func Power(on bool) (bool, error) {
	if board.Current().SoC == board.MediaTek {
		return false, nil
	}
	return sharedhci.Power(on, "/sys/class/rfkill")
}
func BringUp() (*Port, Version, error) {
	if board.Current().SoC != board.MediaTek {
		return sharedhci.BringUp(
			sharedhci.BringUpOptions{
				UART:              TTY,
				FirmwareDirectory: "/bt_firmware/image",
				PatchFile:         "btfwnpla.tlv",
				NVMFile:           "btnvnpla.bin",
				Power:             Power,
				ListenAfterReset:  Listen,
			},
		)
	}
	return sharedhci.BringUp(sharedhci.BringUpOptions{Node: Node})
}

var get = sync.OnceValue(
	func() *Radio { return sharedhci.NewRadio(sharedhci.RadioOptions{BringUp: BringUp, Power: Power}) },
)

func Get() *Radio { return get() }
func init() {
	component.Register(
		component.Hardware,
		Get,
		component.Order(30),
		component.Supervise(service.Restart(2*time.Second, time.Minute)),
	)
}
