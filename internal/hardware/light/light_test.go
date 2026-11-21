package light

import "testing"

func TestEachSensorsRisesNarrowFromClosestToFarthest(t *testing.T) {
	for name, s := range map[string]Sensor{"VCNL4200": &VCNL4200{}, "LTR-578": &LTR578{}} {
		r := s.Rises()
		for i := 1; i < len(r); i++ {
			if r[i] <= 0 || r[i] >= r[i-1] {
				t.Errorf("%s: rises %v, want each step positive and below the one before", name, r)
				break
			}
		}
	}
}
