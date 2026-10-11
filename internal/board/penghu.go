package board

var Penghu = Board{
	NativeAPI:    27,
	Name:         "penghu",
	Model:        "Lenovo ThinkSmart View",
	SoC:          Qualcomm,
	PanelWidth:   800,
	PanelHeight:  1280,
	CameraWidth:  Blueberry.CameraWidth,
	CameraHeight: Blueberry.CameraHeight,
	SubWidth:     Blueberry.SubWidth,
	SubHeight:    Blueberry.SubHeight,
	Motion:       true,
	Mounted:      90,
	UISize:       "medium",
	Diagonal:     8,
	Tessera:      Tessera{Board: "jc8012p4a1", Columns: 6, Rows: 4},
	Touch:        "himax-touchscreen",
	Amp:          Blueberry.Amp,
	Keys: map[uint16]string{
		115: "volume up",
		114: "volume down",
		91:  "mic mute",
		87:  "camera shutter",
	},
	// TODO: confirm mic mute / camera shutter polarity via getevent; MicSensitivity from the OEM app; CameraTurn on camera bring-up
}

func init() { register(Penghu) }
