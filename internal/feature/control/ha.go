package control

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
)

const haWait = 10 * time.Second

func homeAssistant() []*cobra.Command {
	ha := group("ha", "Ask Home Assistant", "")
	ha.AddCommand(
		says(&cobra.Command{
			Use:   "access",
			Short: "Whether Home Assistant answers actions from this device",
			Args:  cobra.NoArgs,
		}, func([]string) (string, error) {
			return homeassistant.Get().Access().String(), nil
		}),
		says(&cobra.Command{
			Use:   "entities [DOMAIN...]",
			Short: "The entities Home Assistant has, in every domain or the ones named",
		}, func(args []string) (string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), haWait)
			defer cancel()
			got, err := homeassistant.Get().Entities(ctx, homeassistant.Filter{Domains: args})
			if err != nil {
				return "", err
			}
			var out strings.Builder
			for _, e := range got {
				fmt.Fprintf(&out, "%s\t%s\t%s\t%s\n", e.ID, e.State, e.Area, e.Name)
			}
			return out.String(), nil
		}),
		says(&cobra.Command{
			Use:   "labels",
			Short: "The labels Home Assistant has",
			Args:  cobra.NoArgs,
		}, func([]string) (string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), haWait)
			defer cancel()
			got, err := homeassistant.Get().Labels(ctx)
			if err != nil {
				return "", err
			}
			var out strings.Builder
			for _, l := range got {
				fmt.Fprintf(&out, "%s\t%s\n", l.ID, l.Name)
			}
			return out.String(), nil
		}),
		says(&cobra.Command{
			Use:   "light on|off ENTITY",
			Short: "Turn a light on or off",
			Args:  cobra.ExactArgs(2),
		}, func(args []string) (string, error) {
			action := map[string]string{"on": "light.turn_on", "off": "light.turn_off"}[args[0]]
			if action == "" {
				return "", fmt.Errorf("on or off, not %q", args[0])
			}
			ctx, cancel := context.WithTimeout(context.Background(), haWait)
			defer cancel()
			if err := homeassistant.Get().Call(ctx, action, map[string]string{"entity_id": args[1]}); err != nil {
				return "", err
			}
			return args[1] + " " + args[0], nil
		}),
	)
	return []*cobra.Command{ha}
}
