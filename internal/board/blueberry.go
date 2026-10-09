package board

var Blueberry = Board{
	NativeAPI:      27,
	Name:           "blueberry",
	Model:          "Lenovo Smart Display 10",
	SoC:            Qualcomm,
	PanelWidth:     1200,
	PanelHeight:    1920,
	CameraWidth:    1600,
	CameraHeight:   1200,
	SubWidth:       640,
	SubHeight:      480,
	CameraTurn:     3,
	Motion:         true,
	Mounted:        90,
	UISize:         "large",
	Diagonal:       10.03,
	Tessera:        Tessera{Board: "jc8012p4a1", Columns: 8, Rows: 5},
	Touch:          "goodix-ts",
	Amp:            &Chip{Bus: 1, Addr: 0x49},
	AmpPins:        []int{68},
	MicSensitivity: -39,
	MaxFPS:         60,
	SecureDecoders: map[string]string{
		"video/x-vnd.on2.vp9": "OMX.qcom.video.decoder.vp9.secure",
		"video/avc":           "OMX.qcom.video.decoder.avc.secure",
		"video/hevc":          "OMX.qcom.video.decoder.hevc.secure",
	},
	Buttons: []Button{
		{"volume up", 85, false},
		{"volume down", 1019, false},
		{"mic mute", 86, false},
		{"camera shutter", 87, true},
	},
}

func init() { register(Blueberry) }
