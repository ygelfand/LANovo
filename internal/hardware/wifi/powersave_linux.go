//go:build linux

package wifi

import (
	"fmt"
	"net"

	"github.com/mdlayher/genetlink"
	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
)

// The nl80211 family id is assigned when the driver registers.
const nl80211Family = "nl80211"

// The full MAC driver reads power save when it associates; a change on a live association is ignored.
func SetPowerSave(iface string, on bool) error {
	link, err := net.InterfaceByName(iface)
	if err != nil {
		return fmt.Errorf("wifi: %s: %w", iface, err)
	}

	conn, err := genetlink.Dial(nil)
	if err != nil {
		return fmt.Errorf("wifi: generic netlink: %w", err)
	}
	defer conn.Close()

	family, err := conn.GetFamily(nl80211Family)
	if err != nil {
		return fmt.Errorf("wifi: %s: %w", nl80211Family, err)
	}

	state := uint32(unix.NL80211_PS_DISABLED)
	if on {
		state = unix.NL80211_PS_ENABLED
	}

	enc := netlink.NewAttributeEncoder()
	enc.Uint32(unix.NL80211_ATTR_IFINDEX, uint32(link.Index))
	enc.Uint32(unix.NL80211_ATTR_PS_STATE, state)

	data, err := enc.Encode()
	if err != nil {
		return fmt.Errorf("wifi: encoding the request: %w", err)
	}

	_, err = conn.Execute(
		genetlink.Message{
			Header: genetlink.Header{
				Command: unix.NL80211_CMD_SET_POWER_SAVE,
				Version: family.Version,
			},
			Data: data,
		},
		family.ID,
		netlink.Request|netlink.Acknowledge,
	)
	if err != nil {
		return fmt.Errorf("wifi: setting power save on %s: %w", iface, err)
	}
	return nil
}
