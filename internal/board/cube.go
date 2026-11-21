package board

var Cube = Board{
	NativeAPI:   27,
	Name:        "cube",
	Model:       "Lenovo Smart Clock 1",
	SoC:         MediaTek,
	PanelWidth:  480,
	PanelHeight: 800,
	Mounted:     270,
	UISize:      "mini",
	Touch:       []string{"mtk-tpd"},
	Buttons: []Button{
		{"volume up", 429, true},
		{"volume down", 576, true},
		{"mic mute", 410, true},
	},
	Amp:            &Chip{Bus: 2, Addr: 0x2c, Table: "cube"},
	MicADC:         &Chip{Bus: 1, Addr: 0x1b, Enable: 411, Reset: 411, Table: "cube"},
	MicSensitivity: -35,
	// cube mounts / from a read-only dm-verity device (root=/dev/dm-0).
	Root: "/data/misc/lanovo",
}

func init() { register(Cube) }
