package board

var Ivy = Board{
	Name:         "ivy",
	Model:        "Lenovo Smart Display 7",
	SoC:          MediaTek,
	PanelWidth:   600,
	PanelHeight:  1024,
	CameraWidth:  1280,
	CameraHeight: 720,
	SubWidth:     768,
	SubHeight:    432,
	Motion:       false,
	CameraMirror: true,
	Mounted:      270,
	UISize:       "compact",
	Touch:        "fts_ts",
	Buttons: []Button{
		{"volume up", 429, true},
		{"volume down", 576, true},
		{"mic mute", 492, true},
		{"camera shutter", 496, false},
	},
	Amp:    &Chip{Bus: 1, Addr: 0x2d, Enable: 397},
	MicADC: &Chip{Bus: 1, Addr: 0x1b, Enable: 494, Reset: 406},
	SecureDecoders: map[string]string{
		"video/avc":  "OMX.MTK.VIDEO.DECODER.AVC.secure",
		"video/hevc": "OMX.MTK.VIDEO.DECODER.HEVC.secure",
	},
}

func init() { register(Ivy) }
