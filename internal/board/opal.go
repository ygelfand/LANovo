package board

var Opal = Board{
	NativeAPI:      27,
	Name:           "opal",
	Model:          "JBL Link View",
	SoC:            Qualcomm,
	PanelWidth:     800,
	PanelHeight:    1280,
	CameraWidth:    Blueberry.CameraWidth,
	CameraHeight:   Blueberry.CameraHeight,
	SubWidth:       Blueberry.SubWidth,
	SubHeight:      Blueberry.SubHeight,
	CameraTurn:     1,
	Mounted:        90,
	Diagonal:       8,
	Tessera:        Tessera{Board: "jc8012p4a1", Columns: 6, Rows: 4},
	Touch:          "fts_ts",
	AmpPins:        []int{127, 98, 0},
	Stereo:         true,
	MaxFPS:         60,
	SecureDecoders: Blueberry.SecureDecoders,
	Buttons:        Blueberry.Buttons,
}

func init() { register(Opal) }
