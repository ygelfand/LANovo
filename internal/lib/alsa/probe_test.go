package alsa

import (
	"os"
	"testing"
)

func TestProbeMixer(t *testing.T) {
	if os.Getenv("ALSA_PROBE") == "" {
		t.Skip("set ALSA_PROBE=1 on a device with a sound card")
	}

	m, err := OpenMixer(0)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	controls, err := m.Controls()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf(
		"elem_value is %d bytes, elem_info %d; %d controls",
		elemValueSize,
		elemInfoSize,
		len(controls),
	)

	var integers, enums, multi int
	for _, c := range controls {
		v, err := m.Get(c)
		if err != nil {
			t.Errorf("%s: %v", c.Name, err)
			continue
		}
		switch c.Type {
		case TypeEnumerated:
			enums++
			if len(v) > 0 && int(v[0]) >= len(c.Items) {
				// The driver reports an out-of-range item for controls nothing has set.
				t.Logf("%s reports item %d with only %d items", c.Name, v[0], len(c.Items))
			}
		case TypeInteger, TypeBoolean:
			integers++
		}

		if c.Count > 1 {
			multi++
			t.Logf("%-28s type %d count %d value %v", c.Name, c.Type, c.Count, v)
		}
	}
	t.Logf("read %d integer or boolean controls, %d enumerated, %d with several values",
		integers, enums, multi)

	for _, name := range []string{"Right Channel Only", "Audio_DacMux_Setting"} {
		c, err := m.Find(name)
		if err != nil {
			t.Logf("%s: %v", name, err)
			continue
		}
		v, err := m.Get(c)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		t.Logf("%-24s type %d count %d value %v items %v", c.Name, c.Type, c.Count, v, c.Items)
	}
}
