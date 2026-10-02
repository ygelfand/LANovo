package display

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Backlight is the panel's brightness control. SurfaceFlinger and the lights HAL normally drive
// this; with both stopped, nothing turns the panel on and a drawn frame is invisible.
//
// The display controller's own node rather than the PMIC's wled underneath it. Writing wled goes
// behind the controller's back, and bringing the panel up pushes its idea of the level back down,
// wiping anything set beforehand — a panel that stays dark until something writes again.
const Backlight = "/sys/class/leds/lcd-backlight/brightness"

// DefaultBrightness is what the boot screen comes up at, before anything has read a setting.
const DefaultBrightness = 70

// SetBacklight sets brightness as a percentage of the driver's maximum.
func SetBacklight(percent int) error {
	full, err := backlightMax()
	if err != nil {
		return err
	}

	percent = min(max(percent, 0), 100)
	level := full * percent / 100
	if err := os.WriteFile(Backlight, []byte(strconv.Itoa(level)), 0o644); err != nil {
		return fmt.Errorf("backlight: %w", err)
	}
	return nil
}

func backlightMax() (int, error) {
	b, err := os.ReadFile(strings.TrimSuffix(Backlight, "brightness") + "max_brightness")
	if err != nil {
		return 0, fmt.Errorf("backlight max: %w", err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("backlight max: %q", strings.TrimSpace(string(b)))
	}
	return n, nil
}
