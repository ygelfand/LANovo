package layout

import (
	"fmt"
	"strings"
)

// Set by the linker.
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

func VersionString() string {
	return fmt.Sprintf("%s (%s, built %s)", Version, GitCommit, BuildDate)
}

// A service takes its SELinux domain from the label of the file init execs.
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

	// / is read-only; /data is unencrypted.
	StateDir = "/data/misc/lanovo"

	Stash = "/data/misc/lanovo/stash"

	TempDir = "/data/misc/lanovo/tmp"

	LogTag = "lanovod"

	StatePath = StateDir + "/state.json"

	NamePath = StateDir + "/name"

	KeyPath = StateDir + "/psk"

	CertDirs = "/system/etc/security/cacerts:/data/misc/keychain/certs-added"

	CastCredentialsPath = StateDir + "/cast-credentials.json"

	CastAuthorityPath = StateDir + "/cast-authority.json"

	CastAppDir = StateDir + "/cast"

	// init sends stderr to /dev/null.
	CrashPath = StateDir + "/crash"

	BondPath = StateDir + "/bonds"

	ModelDir = StateDir + "/models"

	RecordingDir = StateDir + "/recordings"
)

const DefaultName = "Smart Display"

// wpa_supplicant drops to WifiUser; the vendor's wpa_supplicant.conf is not usable standalone.
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

const (
	Port       = 6053
	ListenAddr = ":6053"
)

const (
	Manufacturer = "LANovo"
	Model        = "LANovo"
	Board        = "apq8053"
	Platform     = "lanovo"
)

// The ESPHome API's node name length limit.
const MaxNodeName = 31

const FBDevice = "/dev/graphics/fb0"

const (
	IonDevice     = "/dev/ion"
	DispMgrDevice = "/dev/mtk_disp_mgr"
)

// lcd-backlight drives the same panel at 0..255.
const (
	Backlight    = "/sys/class/leds/wled/brightness"
	BacklightMax = 4095
)

// Buttons and the mic slider are active high; the camera shutter reads 1 when open.
const (
	GPIOVolumeUp    = 85
	GPIOVolumeDown  = 1019 // on the PMIC, not the SoC
	GPIOMicMute     = 86
	GPIOCameraCover = 87
	GPIORotateMic   = 117
	GPIOAmpEnable   = 68
)

// Stopping zygote takes the sparrow OEM app, which holds the buttons and I2C2.
var Displace = []string{
	"zygote",
	"audioserver",
	"audio-hal-2-0",
	"update_engine",
	"peripheralman",
	"inputdriverserv",
}

// Exists only once the driver is loaded.
const MACPath = "/sys/class/net/" + WifiIface + "/address"

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
