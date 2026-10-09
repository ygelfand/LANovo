package board

import (
	"errors"
	"io/fs"
	"testing"
)

type fakeFiles map[string]string

func (f fakeFiles) ReadFile(path string) ([]byte, error) {
	s, ok := f[path]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return []byte(s), nil
}

type fakeProps map[string]string

func (p fakeProps) Getprop(name string) (string, error) { return p[name], nil }
func (p fakeProps) Setprop(name, value string) error    { p[name] = value; return nil }

func device(mode string, names ...string) fakeFiles {
	inputs := ""
	for _, n := range names {
		inputs += "I: Bus=0000 Vendor=0000 Product=0000 Version=0000\nN: Name=\"" + n + "\"\nP: Phys=\n\n"
	}
	return fakeFiles{ModesPath: mode + "\n", InputsPath: inputs}
}

func TestEachBoardIsFoundByItsHardware(t *testing.T) {
	for _, c := range []struct {
		files fakeFiles
		want  string
	}{
		{device("U:1200x1920p-60", "qpnp_pon", "goodix-ts", "msm8953-openq624-snd-card Headset Jack"), "blueberry"},
		{device("U:800x1280p-60", "qpnp_pon", "fts_ts", "msm8953-openq624-snd-card Button Jack"), "opal"},
		{device("U:800x1280p-60", "qpnp_pon", "goodix-ts"), "amber"},
		{device("U:600x1024p-0", "mtk-pmic-keys", "fts_ts", "mtk-tpd"), "ivy"},
	} {
		b, err := Resolve(fakeProps{}, c.files)
		if err != nil || b.Name != c.want {
			t.Errorf("found %q, %v; want %s", b.Name, err, c.want)
		}
	}
}

func TestNoTwoBoardsShareTheirHardware(t *testing.T) {
	for i, a := range boards {
		for _, b := range boards[i+1:] {
			if a.PanelWidth == b.PanelWidth && a.PanelHeight == b.PanelHeight &&
				a.Touch == b.Touch {
				t.Errorf("%s and %s cannot be told apart", a.Name, b.Name)
			}
		}
	}
}

func TestTheRecordedBoardWins(t *testing.T) {
	b, err := Resolve(fakeProps{"ro.lanovo.board": "opal"}, fakeFiles{})
	if err != nil || b.Name != "opal" {
		t.Fatalf("found %q, %v", b.Name, err)
	}
	if _, err := Resolve(
		fakeProps{"ro.lanovo.board": "pearl"},
		device("U:1200x1920p-60", "goodix-ts"),
	); err == nil {
		t.Error("an unknown recorded board was accepted")
	}
}

func TestUnknownHardwareIsAnError(t *testing.T) {
	if _, err := Resolve(fakeProps{}, device("U:720x1280p-60", "goodix-ts")); err == nil {
		t.Error("a panel no board has was accepted")
	}
	if _, err := Resolve(fakeProps{}, fakeFiles{}); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing files gave %v", err)
	}
}
