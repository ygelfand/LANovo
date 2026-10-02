package lanovod

import (
	"fmt"
	"net"

	"github.com/spf13/cobra"

	"github.com/ygelfand/LANovo/internal/feature/dhcp"
)

func newProbeCmd() *cobra.Command {
	var iface string

	c := &cobra.Command{
		Use:   "probe ADDRESS",
		Short: "Ask whether anything on the network already holds an address",
		Long: "The RFC 5227 check the DHCP client runs before it takes a new address: three ARP\n" +
			"probes, and whatever answers. Point it at the router to see a conflict reported, and\n" +
			"at a free address to see one go unanswered — which is the only way to tell the two\n" +
			"apart on real hardware, since the client is silent when the address is free.",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ip := net.ParseIP(args[0])
			if ip == nil || ip.To4() == nil {
				return fmt.Errorf("%q is not an IPv4 address", args[0])
			}

			taken, err := dhcp.Probe(iface, ip)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if taken {
				fmt.Fprintf(out, "%s is taken\n", ip)
				return nil
			}
			fmt.Fprintf(out, "%s went unanswered\n", ip)
			return nil
		},
	}

	c.Flags().StringVar(&iface, "interface", dhcp.Interface, "which interface to probe from")
	return c
}
