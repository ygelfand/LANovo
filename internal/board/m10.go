package board

var M10 = func() Board {
	b := Blueberry
	b.Name = "m10"
	b.Model = "Lenovo Smart Display M10"
	b.PanelWidth = Amber.PanelWidth
	b.PanelHeight = Amber.PanelHeight
	b.UISize = Amber.UISize
	b.Tessera = Amber.Tessera
	b.Diagonal = 10.1
	b.CameraWidth, b.CameraHeight = 1920, 1080
	b.CameraSizes = []string{"1920x1080", "1280x720", "640x480"}
	b.Amp = nil
	b.AmpPins = nil
	b.SpeakerRoute = []Mixer{
		{Name: "QUAT_MI2S_RX Audio Mixer MultiMedia1", Value: 1},
		{Name: "MI2S_RX Channels", Choice: "One"},
		{Name: "RX3 MIX1 INP1", Choice: "RX1"},
		{Name: "RX3 Digital Volume", Value: RXUnity},
		{Name: "WSA Spk Switch", Choice: "ZERO"},
		{Name: "SPK", Choice: "Switch"},
	}
	return b
}()

func init() { register(M10) }
