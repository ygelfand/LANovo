// Package layout is the on-device layout lanovoctl writes and lanovod reads.
package layout

import (
	"fmt"
	"strings"
)

// Set by the linker. See the Makefile.
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// VersionString is what both binaries report for --version.
func VersionString() string {
	return fmt.Sprintf("%s (%s, built %s)", Version, GitCommit, BuildDate)
}

// Where lanovod lives.
//
// /system/bin, not /data: a service takes its SELinux domain from the label of the file init
// execs, /system/bin is system_file, and system_file has no transition rule, so the service stays
// in init's domain. A /data label init may refuse to exec.
const (
	Binary  = "/system/bin/lanovod"
	InitRC  = "/system/etc/init/lanovod.rc"
	Service = "lanovod"

	Surface        = "/system/bin/lanovo-surface"
	SurfaceSocket  = "/dev/socket/lanovo-surface"
	SurfaceService = "lanovo_surface"

	CamShim             = "/system/lib/liblanovo-camshim.so"
	CameraHALService    = "lanovo_camhal"
	CameraServerService = "lanovo_camserver"
	CameraService       = "lanovo_camera"
	Camera              = "/system/bin/lanovo-camera"
	CameraSocket        = "/dev/socket/lanovo-camera"

	PrevBinary = Binary + ".prev"
	Incoming   = StateDir + "/lanovod.incoming"

	// StateDir is everything written after install. / is read-only with 57MB free; /data has the
	// room and is unencrypted.
	StateDir = "/data/misc/lanovo"

	// Stash is where the system apps moved off / to make room are kept.
	Stash = "/data/misc/lanovo/stash"

	TempDir = "/data/misc/lanovo/tmp"

	// LogTag is lanovod's logcat tag: `adb logcat -s lanovod`.
	LogTag = "lanovod"

	// StatePath holds everything the device is set to.
	StatePath = StateDir + "/state.json"

	// NamePath holds the display name chosen at install.
	NamePath = StateDir + "/name"

	// KeyPath holds the ESPHome encryption key Home Assistant pairs with.
	KeyPath = StateDir + "/psk"

	// CertDirs are the platform's root certificates and any the user added.
	CertDirs = "/system/etc/security/cacerts:/data/misc/keychain/certs-added"

	// CastCredentialsPath holds the last answer from the cast oracle.
	CastCredentialsPath = StateDir + "/cast-credentials.json"

	// CastAuthorityPath holds the device's own certificate chain and key, made once.
	CastAuthorityPath = StateDir + "/cast-authority.json"

	CastAppDir = StateDir + "/cast"

	// CrashPath holds the Go runtime's report of a run that died; init sends stderr to /dev/null.
	CrashPath = StateDir + "/crash"

	// BondPath holds the Bluetooth link keys of the phones that have paired. Beside the psk rather
	// than in state.json: these are secrets, and the settings are read back out over the API.
	BondPath = StateDir + "/bonds"

	// ModelDir holds the wake word models. In /data rather than /system: Home Assistant can offer
	// new ones at runtime and / is mounted read-only.
	ModelDir = StateDir + "/models"

	// RecordingDir holds the audio of turns, for the assistants set to keep any. In /data because it
	// grows and is pruned, and because / is mounted read-only.
	RecordingDir = StateDir + "/recordings"
)

// DefaultName is what a device nobody named calls itself.
const DefaultName = "Smart Display"

// The supplicant, and where it keeps its configuration and control socket. It runs as init's
// service, defined in InitRC, which init learns by reading at boot.
//
// wpa_supplicant drops to WifiUser, so WifiConf is wifi:wifi 0660. It is our own file: the
// vendor's wpa_supplicant.conf beside it is not usable standalone.
const (
	Supplicant        = "/vendor/bin/hw/wpa_supplicant"
	SupplicantService = "lanovo_wpa"

	WifiUser    = 1010
	WifiDir     = "/data/misc/wifi"
	WifiConf    = WifiDir + "/lanovo_wpa.conf"
	WifiBgscan  = "simple:30:-65:300"
	WifiSockets = WifiDir + "/sockets"
	WifiIface   = "wlan0"
)

// Port is the ESPHome native API port Home Assistant expects, and ListenAddr is where the server
// binds.
const (
	Port       = 6053
	ListenAddr = ":6053"
)

// Hardware identity, as Home Assistant shows it.
const (
	Manufacturer = "LANovo"
	Model        = "LANovo"
	Board        = "apq8053"
	Platform     = "lanovo"
)

// MaxNodeName is the length limit the ESPHome API imposes on a node name.
const MaxNodeName = 31

const FBDevice = "/dev/graphics/fb0"

const (
	IonDevice     = "/dev/ion"
	DispMgrDevice = "/dev/mtk_disp_mgr"
)

// Backlight brightness. lcd-backlight drives the same panel at 0..255.
const (
	Backlight    = "/sys/class/leds/wled/brightness"
	BacklightMax = 4095
)

// The GPIO lines, named as Lenovo's OEM driver names them.
//
// Buttons and the mic slider are active high. The camera shutter reads 1 when OPEN.
const (
	GPIOVolumeUp    = 85
	GPIOVolumeDown  = 1019 // on the PMIC, not the SoC
	GPIOMicMute     = 86
	GPIOCameraCover = 87
	GPIORotateMic   = 117
	GPIOAmpEnable   = 68
)

// Displace is what has to let go before lanovod can have the device. zygote takes system_server,
// the Assistant launcher, Cast, the Things UI and the sparrow OEM app with it; the OEM app holds
// the buttons and I2C2.
var Displace = []string{
	"zygote",
	"audioserver",
	"audio-hal-2-0",
	"update_engine",
	"peripheralman",
	"inputdriverserv",
}

// MACPath is the wlan0 address, which is the device's identity to Home Assistant. It exists only
// once the driver has been loaded.
const MACPath = "/sys/class/net/" + WifiIface + "/address"

// MAC normalizes an address into the form Home Assistant compares against, and reports "" for
// anything that would not identify a device.
func MAC(raw string) string {
	var digits strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(raw)) {
		if r >= '0' && r <= '9' || r >= 'a' && r <= 'f' {
			digits.WriteRune(r)
		}
	}

	s := digits.String()
	if len(s) != 12 || s == "000000000000" {
		return ""
	}

	var mac strings.Builder
	for i := 0; i < len(s); i += 2 {
		if i > 0 {
			mac.WriteByte(':')
		}
		mac.WriteString(s[i : i+2])
	}
	return mac.String()
}

// NameFromMAC builds the fallback display name, unique per device.
func NameFromMAC(mac string) string {
	var hex strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(mac)) {
		if r >= '0' && r <= '9' || r >= 'A' && r <= 'F' {
			hex.WriteRune(r)
		}
	}

	s := hex.String()
	if len(s) < 6 {
		return DefaultName
	}
	return DefaultName + " " + s[len(s)-6:]
}

// Slug is the node name a display name becomes: the mDNS hostname and the prefix of every entity
// id.
func Slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case b.Len() > 0 && !dash:
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}
