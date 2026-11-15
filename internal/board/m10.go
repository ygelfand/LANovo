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
	b.CameraTurn = 2
	return b
}()

func init() { register(M10) }
