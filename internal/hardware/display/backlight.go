package display

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Writing the PMIC's wled directly is reset when the controller brings the panel up.
const Backlight = "/sys/class/leds/lcd-backlight/brightness"

const DefaultBrightness = 70

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
