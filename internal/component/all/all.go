// Package all is what this device is made of.
//
// Components register themselves from init, so a component nobody imports is a component that
// silently does not exist — no entity, no lifecycle, no error. This is the one list that pulls them
// in, and importing it is what makes the registry complete.
//
// Order is not decided here. Each component declares its phase and its place within it, so this list
// can stay alphabetical and mean nothing but membership. Hardware is listed as well as features:
// relying on a feature to pull its driver in transitively is a component that disappears the moment
// the feature stops importing it.
package all

import (
	_ "github.com/ygelfand/LANovo/internal/feature/a2dp"
	_ "github.com/ygelfand/LANovo/internal/feature/access"
	_ "github.com/ygelfand/LANovo/internal/feature/activity"
	_ "github.com/ygelfand/LANovo/internal/feature/api"
	_ "github.com/ygelfand/LANovo/internal/feature/assistant"
	_ "github.com/ygelfand/LANovo/internal/feature/bluetooth"
	_ "github.com/ygelfand/LANovo/internal/feature/buttons"
	_ "github.com/ygelfand/LANovo/internal/feature/chromecast"
	_ "github.com/ygelfand/LANovo/internal/feature/clock"
	_ "github.com/ygelfand/LANovo/internal/feature/control"
	_ "github.com/ygelfand/LANovo/internal/feature/dashboard"
	_ "github.com/ygelfand/LANovo/internal/feature/detect"
	_ "github.com/ygelfand/LANovo/internal/feature/dhcp"
	_ "github.com/ygelfand/LANovo/internal/feature/diag"
	_ "github.com/ygelfand/LANovo/internal/feature/drawer"
	_ "github.com/ygelfand/LANovo/internal/feature/drm"
	_ "github.com/ygelfand/LANovo/internal/feature/feedback"
	_ "github.com/ygelfand/LANovo/internal/feature/firmware"
	_ "github.com/ygelfand/LANovo/internal/feature/gui"
	_ "github.com/ygelfand/LANovo/internal/feature/homeassistant"
	_ "github.com/ygelfand/LANovo/internal/feature/media"
	_ "github.com/ygelfand/LANovo/internal/feature/message"
	_ "github.com/ygelfand/LANovo/internal/feature/microphone"
	_ "github.com/ygelfand/LANovo/internal/feature/network"
	_ "github.com/ygelfand/LANovo/internal/feature/noise"
	_ "github.com/ygelfand/LANovo/internal/feature/poster"
	_ "github.com/ygelfand/LANovo/internal/feature/privacy"
	_ "github.com/ygelfand/LANovo/internal/feature/reboot"
	_ "github.com/ygelfand/LANovo/internal/feature/recording"
	_ "github.com/ygelfand/LANovo/internal/feature/rtspd"
	_ "github.com/ygelfand/LANovo/internal/feature/screen"
	_ "github.com/ygelfand/LANovo/internal/feature/sendspin"
	_ "github.com/ygelfand/LANovo/internal/feature/sensors"
	_ "github.com/ygelfand/LANovo/internal/feature/settings"
	_ "github.com/ygelfand/LANovo/internal/feature/shell"
	_ "github.com/ygelfand/LANovo/internal/feature/states"
	_ "github.com/ygelfand/LANovo/internal/feature/timer"
	_ "github.com/ygelfand/LANovo/internal/feature/viewassist"
	_ "github.com/ygelfand/LANovo/internal/feature/vision"
	_ "github.com/ygelfand/LANovo/internal/feature/visuals"
	_ "github.com/ygelfand/LANovo/internal/feature/voice"
	_ "github.com/ygelfand/LANovo/internal/feature/volume"
	_ "github.com/ygelfand/LANovo/internal/feature/wakeword"
	_ "github.com/ygelfand/LANovo/internal/feature/web"
	_ "github.com/ygelfand/LANovo/internal/hardware/ble"
	_ "github.com/ygelfand/LANovo/internal/hardware/buttons"
	_ "github.com/ygelfand/LANovo/internal/hardware/display"
	_ "github.com/ygelfand/LANovo/internal/hardware/mic"
	_ "github.com/ygelfand/LANovo/internal/hardware/speaker"
	_ "github.com/ygelfand/LANovo/internal/hardware/touch"
	_ "github.com/ygelfand/LANovo/internal/hardware/wifi"
)
