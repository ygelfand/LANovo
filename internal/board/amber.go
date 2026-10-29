package board

import "github.com/ygelfand/libcountertop/pkg/bluetooth/hci"

var Amber = Board{
	NativeAPI:    27,
	Name:         "amber",
	Model:        "Lenovo Smart Display 8",
	SoC:          Qualcomm,
	Bluetooth:    hci.ControllerSetsParams,
	PanelWidth:   800,
	PanelHeight:  1280,
	CameraWidth:  Blueberry.CameraWidth,
	CameraHeight: Blueberry.CameraHeight,
	CameraSizes:  Blueberry.CameraSizes,
	SubWidth:     Blueberry.SubWidth,
	SubHeight:    Blueberry.SubHeight,
	CameraTurn:   Blueberry.CameraTurn,
	Motion:       true,
	Mounted:      90,
	UISize:       "medium",
	Diagonal:     8,
	Tessera:      Tessera{Board: "jc8012p4a1", Columns: 6, Rows: 4},
	Touch:        []string{"himax-touchscreen"},
	Thermal:      Blueberry.Thermal,
	SoundCard:    "msm8953-openq624-snd-card",
	Buttons:      Blueberry.Buttons,
	Amp:          &Chip{Bus: 1, Addr: 0x49, Table: "amber"},
	AmpPins:      Blueberry.AmpPins,
	// Inferred from blueberry's hardware; not yet tested on amber.
	MicSensitivity: Blueberry.MicSensitivity,
}

func init() { register(Amber) }
