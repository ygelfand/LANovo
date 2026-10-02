package board

var Blueberry = Board{
	Name:         "blueberry",
	Model:        "Lenovo Smart Display 10",
	SoC:          Qualcomm,
	PanelWidth:   1200,
	PanelHeight:  1920,
	CameraWidth:  1600,
	CameraHeight: 1200,
	SubWidth:     640,
	SubHeight:    480,
	Motion:       true,
	Mounted:      90,
	UISize:       "large",
	Touch:        "goodix-ts",
	Buttons: []Button{
		{"volume up", 85, false},
		{"volume down", 1019, false},
		{"mic mute", 86, false},
		{"camera shutter", 87, true},
	},
}

func init() { register(Blueberry) }
