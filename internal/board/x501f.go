package board

// X501F uses an 800x1280 panel with Goodix touch and the openq624 ALSA card.
// The amplifier GPIOs and register table must be confirmed on the stock firmware.
var X501F = Board{
	NativeAPI:   27,
	Motion:      true,
	Name:        "sd-x501f",
	Model:       "Lenovo Smart Display M10",
	SoC:         Qualcomm,
	PanelWidth:  800,
	PanelHeight: 1280,
	// The SD-X501F front module was identified as S5KC505A by the Android 7
	// factory camera test and confirmed working on Android 8.1 after adding its
	// camera config and tuning libraries. Keep this board profile tied to that
	// module; the unmodified Android 8 image only listed IMX307/IMX214/OV5693.
	// 1920x1080 is the active image size used by the Qualcomm camera stack.
	CameraWidth:  1920,
	CameraHeight: 1080,
	CameraTurn:   2,
	CameraSizes:  []string{"1920x1080", "1280x720", "640x480"},
	SubWidth:     640,
	SubHeight:    480,
	Mounted:      90,
	UISize:       "medium",
	Touch:        []string{"goodix-ts"},
	SoundCard:    "msm8953-openq624-snd-card",
	// GPIO 68 is the Qualcomm Smart Display speaker-enable line; the SD-X501F
	// amp register table is intentionally unset until its exact chip is identified.
	AmpPins: []int{68},
	// Physical buttons were confirmed active-high on these lines on one SD-X501F test device.
	Buttons: []Button{
		{"volume up", 85, false},
		{"volume down", 1019, false},
	},
}

func init() { register(X501F) }
