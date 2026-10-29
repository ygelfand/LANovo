package board

import "github.com/ygelfand/libcountertop/pkg/bluetooth/hci"

var Blueberry = Board{
	NativeAPI:    27,
	Name:         "blueberry",
	Model:        "Lenovo Smart Display 10",
	SoC:          Qualcomm,
	Bluetooth:    hci.ControllerSetsParams,
	PanelWidth:   1200,
	PanelHeight:  1920,
	CameraWidth:  1600,
	CameraHeight: 1200,
	CameraSizes: []string{
		"1920x1080",
		"1600x1200",
		"1280x960",
		"1024x768",
		"800x600",
		"640x480",
	},
	SubWidth:   640,
	SubHeight:  480,
	CameraTurn: 3,
	Motion:     true,
	Mounted:    90,
	UISize:     "large",
	Diagonal:   10.03,
	Tessera:    Tessera{Board: "jc8012p4a1", Columns: 8, Rows: 5},
	Touch:      []string{"goodix-ts"},
	// deca-cpu-max-step is the SoC's hottest-core reading; throttling follows it.
	Thermal: Thermal{
		CPU:       []string{"deca-cpu-max-step"},
		GPU:       []string{"gpu0-usr"},
		PerDegree: 1000,
	},
	Amp:            &Chip{Bus: 1, Addr: 0x49, Table: "blueberry"},
	AmpPins:        []int{68},
	MicSensitivity: -39,
	MaxFPS:         60,
	DecodeWidth:    1920,
	DecodeHeight:   1088,
	SecureDecoders: map[string]string{
		"video/x-vnd.on2.vp9": "OMX.qcom.video.decoder.vp9.secure",
		"video/avc":           "OMX.qcom.video.decoder.avc.secure",
		"video/hevc":          "OMX.qcom.video.decoder.hevc.secure",
	},
	Buttons: []Button{
		{"volume up", 85, false},
		{"volume down", 1019, false},
		{"mic mute", 86, false},
		{"camera shutter", 87, true},
	},
}

func init() { register(Blueberry) }
