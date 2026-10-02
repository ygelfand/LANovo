package wpa

import "testing"

// The vectors IEEE 802.11i gives for PBKDF2-SHA1 with the network name as salt.
func TestKey(t *testing.T) {
	tests := []struct {
		name       string
		ssid       string
		passphrase string
		want       string
		wantErr    bool
	}{
		{
			name:       "IEEE vector one",
			ssid:       "IEEE",
			passphrase: "password",
			want:       "f42c6fc52df0ebef9ebb4b90b38a5f902e83fe1b135a70e23aed762e9710a12e",
		},
		{
			name:       "IEEE vector two",
			ssid:       "ThisIsASSID",
			passphrase: "ThisIsAPassword",
			want:       "0dc0d6eb90555ed6419756b9a15ec3e3209b63df707dd508d14581f8982721af",
		},
		{
			name:       "a 64 character hex string is already a key",
			ssid:       "anything",
			passphrase: "f42c6fc52df0ebef9ebb4b90b38a5f902e83fe1b135a70e23aed762e9710a12e",
			want:       "f42c6fc52df0ebef9ebb4b90b38a5f902e83fe1b135a70e23aed762e9710a12e",
		},
		{
			name:       "too short for WPA",
			ssid:       "net",
			passphrase: "short",
			wantErr:    true,
		},
		{
			name:       "too long for WPA",
			ssid:       "net",
			passphrase: string(make([]byte, 64)),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Key(tt.ssid, tt.passphrase)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want an error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Key(%q, %q) = %q, want %q", tt.ssid, tt.passphrase, got, tt.want)
			}
		})
	}
}

func TestSecurity(t *testing.T) {
	tests := []struct {
		name  string
		flags string
		want  Security
	}{
		{"open", "[ESS]", Open},
		{"WPA2", "[WPA2-PSK-CCMP][ESS]", PSK},
		{"WPA3 only", "[RSN-SAE-CCMP][ESS]", SAE},
		{"transition mode joins as PSK", "[WPA2-PSK+SAE-CCMP][ESS]", PSK},
		{"enterprise", "[WPA2-EAP-CCMP][ESS]", Enterprise},
		{"fast transition is still PSK", "[WPA2-PSK+FT/PSK-CCMP][ESS]", PSK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SecurityOf(tt.flags); got != tt.want {
				t.Errorf("SecurityOf(%q) = %v, want %v", tt.flags, got, tt.want)
			}
		})
	}
}

func TestSupported(t *testing.T) {
	for _, s := range []Security{Open, PSK} {
		if !s.Supported() {
			t.Errorf("%v should be joinable", s)
		}
	}
	for _, s := range []Security{SAE, Enterprise} {
		if s.Supported() {
			t.Errorf("%v is not joinable by this device's supplicant", s)
		}
	}
}

func TestParse(t *testing.T) {
	const out = "bssid / frequency / signal level / flags / ssid\n" +
		"f0:9f:c2:f5:50:5f\t5240\t-52\t[WPA2-PSK-CCMP][ESS]\tiron.curtain\r\n" +
		"78:45:58:35:71:2a\t5805\t-64\t[WPA2-PSK-CCMP][ESS]\tiron.curtain\r\n" +
		"fa:9f:c2:f5:50:5f\t5240\t-53\t[WPA2-PSK-CCMP][ESS]\t\r\n" +
		"94:4e:5b:38:85:f4\t2412\t-39\t[WPA2-PSK-CCMP][WPS][ESS]\tenud_opt\r\n"

	got := ParseScan(out)

	if len(got) != 2 {
		t.Fatalf("parsed %d networks, want 2: %v", len(got), got)
	}
	if n := got["iron.curtain"]; n.Signal != -52 {
		t.Errorf("kept the %d dBm sighting of iron.curtain, want the strongest at -52", n.Signal)
	}
	if _, ok := got[""]; ok {
		t.Error("an access point hiding its name should not be offered")
	}
	if n := got["enud_opt"]; n.Security != PSK {
		t.Errorf("enud_opt security = %v, want %v", n.Security, PSK)
	}
}

func TestLastLine(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string
	}{
		{"one line", "OK", "OK"},
		{"trailing newline is ignored", "3\n", "3"},
		{"the id follows the chatter", "Using interface wlan0\n3\n", "3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LastLine(tt.out); got != tt.want {
				t.Errorf("LastLine(%q) = %q, want %q", tt.out, got, tt.want)
			}
		})
	}
}

func TestParseStatus(t *testing.T) {
	const out = "bssid=f0:9f:c2:f5:50:5f\nfreq=5240\nssid=iron.curtain\nid=0\n" +
		"mode=station\npairwise_cipher=CCMP\ngroup_cipher=CCMP\nkey_mgmt=WPA2-PSK\n" +
		"wpa_state=COMPLETED\nip_address=10.100.101.106\naddress=00:f4:8d:47:69:21\n"

	got := ParseStatus(out)

	if got.SSID != "iron.curtain" {
		t.Errorf("ssid = %q, want iron.curtain", got.SSID)
	}
	if got.State != "COMPLETED" {
		t.Errorf("state = %q, want COMPLETED", got.State)
	}
	if got.Address != "10.100.101.106" {
		t.Errorf("address = %q, want 10.100.101.106", got.Address)
	}
}

// The supplicant answers status before it has joined anything, and with far fewer keys.
func TestParseStatusDisconnected(t *testing.T) {
	got := ParseStatus("wpa_state=DISCONNECTED\naddress=00:f4:8d:47:69:21\n")

	if got.State != "DISCONNECTED" {
		t.Errorf("state = %q, want DISCONNECTED", got.State)
	}
	if got.SSID != "" || got.Address != "" {
		t.Errorf("a disconnected supplicant reported %q and %q", got.SSID, got.Address)
	}
	if got.Associated() {
		t.Error("DISCONNECTED counts as associated")
	}
}

// address is the MAC and ip_address is the address: taking the wrong one reports a MAC as the
// device's address, which reads as plausible everywhere it is shown.
func TestParseStatusDoesNotTakeTheMAC(t *testing.T) {
	got := ParseStatus("wpa_state=COMPLETED\naddress=00:f4:8d:47:69:21\n")

	if got.Address != "" {
		t.Errorf("address = %q, want empty: only ip_address is an address", got.Address)
	}
}

func TestParseNetworks(t *testing.T) {
	const out = "network id / ssid / bssid / flags\n" +
		"0\tiron.curtain\tany\t[CURRENT]\r\n" +
		"1\tenud_opt\tany\t\r\n"

	got := ParseNetworks(out)

	if len(got) != 2 || got[0] != "iron.curtain" || got[1] != "enud_opt" {
		t.Errorf("parseNetworks = %q, want [iron.curtain enud_opt]", got)
	}
}

// A supplicant with nothing configured answers the header alone, which is not a network.
func TestParseNetworksWhenEmpty(t *testing.T) {
	if got := ParseNetworks("network id / ssid / bssid / flags\n"); len(got) != 0 {
		t.Errorf("parseNetworks = %q, want none", got)
	}
}

// Joined takes a name because COMPLETED alone is still true of the network being left, for a
// second or two after select_network asks to switch.
func TestJoined(t *testing.T) {
	on := State{SSID: "iron.curtain", State: "COMPLETED"}

	if !on.Joined("") {
		t.Error("any network should satisfy an empty name")
	}
	if !on.Joined("iron.curtain") {
		t.Error("the network it is on should satisfy its own name")
	}
	if on.Joined("enud_opt") {
		t.Error("the network being left satisfied the name of the one being joined")
	}

	off := State{SSID: "iron.curtain", State: "4WAY_HANDSHAKE"}
	if off.Joined("") || off.Joined("iron.curtain") {
		t.Error("a handshake still in progress counts as joined")
	}
}
