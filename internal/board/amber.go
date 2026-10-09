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
	CameraTurn:   Blueberry.CameraTurn,
	Motion:       true,
	Mounted:      90,
	Diagonal:     8,
	Tessera:      Tessera{Board: "jc8012p4a1", Columns: 6, Rows: 4},
	Touch:        "goodix-ts",
	Buttons:      Blueberry.Buttons,
	AmpPins:      Blueberry.AmpPins,
}

func init() { register(Amber) }
