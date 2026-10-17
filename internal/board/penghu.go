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
	CameraTurn:   Blueberry.CameraTurn,
	Motion:       true,
	Mounted:      90,
	UISize:       "medium",
	Diagonal:     8,
	Tessera:      Tessera{Board: "jc8012p4a1", Columns: 6, Rows: 4},
	Touch:        "himax-touchscreen",
	// The kernel TAS5782M codec loads its own table on first playback.
	// Only drive the enable pin here; a userspace table load races that work.
	AmpPins:               Blueberry.AmpPins,
	MicNoTertiaryChannels: true,
	// Provisional until Penghu's microphone sensitivity is measured.
	MicSensitivity: Blueberry.MicSensitivity,
	DecodeWidth:    3840,
	DecodeHeight:   2160,
	// aboot power-on write-protects the eMMC group that holds system's superblock.
	Root: "/data/misc/lanovo",
	Buttons: []Button{
		{"volume up", 85, true},
		{"mic mute", 86, false},
		{"camera shutter", 87, true},
	},
	Keys: map[uint16]string{
		114: "volume down",
	},
}

func init() { register(Penghu) }
