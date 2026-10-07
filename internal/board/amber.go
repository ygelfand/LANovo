package board

var Amber = Board{
	NativeAPI:    27,
	Name:         "amber",
	Model:        "Lenovo Smart Display 8",
	SoC:          Qualcomm,
	PanelWidth:   800,
	PanelHeight:  1280,
	CameraWidth:  Blueberry.CameraWidth,
	CameraHeight: Blueberry.CameraHeight,
	SubWidth:     Blueberry.SubWidth,
	SubHeight:    Blueberry.SubHeight,
	Motion:       true,
	Mounted:      90,
	Touch:        "goodix-ts",
	Buttons:      Blueberry.Buttons,
}

func init() { register(Amber) }
