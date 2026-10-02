package wifi

import (
	"fmt"
	"net"

	"github.com/jsimonetti/rtnetlink"
	"golang.org/x/sys/unix"
)

// linkUp brings the interface up.
func linkUp(iface string) error {
	link, err := net.InterfaceByName(iface)
	if err != nil {
		return fmt.Errorf("wifi: %s: %w", iface, err)
	}
	if link.Flags&net.FlagUp != 0 {
		return nil
	}

	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		return fmt.Errorf("wifi: rtnetlink: %w", err)
	}
	defer conn.Close()

	// Change says which flags Flags is about, so this sets IFF_UP and leaves the rest alone.
	if err := conn.Link.Set(&rtnetlink.LinkMessage{
		Family: unix.AF_UNSPEC,
		Index:  uint32(link.Index),
		Flags:  unix.IFF_UP,
		Change: unix.IFF_UP,
	}); err != nil {
		return fmt.Errorf("wifi: bringing %s up: %w", iface, err)
	}
	return nil
}
