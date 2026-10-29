package board

import "github.com/ygelfand/libcountertop/pkg/bluetooth/hci"

var Penghu = Board{
	NativeAPI:    27,
	Name:         "penghu",
	Model:        "Lenovo ThinkSmart View",
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
	Touch:        []string{"himax-touchscreen", "fts_ts"},
	SoundCard:    "msm8953-snd-card-mtp",
	// MSM8953 tsens: 4-7 and 9-12 are the two CPU clusters, 14 and 15 the GPU.
	Thermal: Thermal{
		CPU: []string{
			"tsens_tz_sensor4", "tsens_tz_sensor5", "tsens_tz_sensor6", "tsens_tz_sensor7",
			"tsens_tz_sensor9", "tsens_tz_sensor10", "tsens_tz_sensor11", "tsens_tz_sensor12",
		},
		GPU:       []string{"tsens_tz_sensor14", "tsens_tz_sensor15"},
		PerDegree: 10,
	},
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
