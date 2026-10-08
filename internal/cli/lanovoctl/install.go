package lanovoctl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ygelfand/libcountertop/pkg/host/adb"
	"github.com/ygelfand/libcountertop/pkg/host/prompt"
	"github.com/ygelfand/libcountertop/pkg/host/wifisetup"

	"github.com/ygelfand/LANovo/internal/host/takeover"
)

func newInstallCmd() *cobra.Command {
	var (
		binary   string
		name     string
		yes      bool
		noBoot   bool
		ssid     string
		password string
		wps      bool
		doReboot bool
		noReboot bool
		genKey   bool
	)

	c := &cobra.Command{
		Use:   "install",
		Short: "Install LANovo on a display",
		Long: "Patches the boot command line for permissive SELinux, then installs lanovod and\n" +
			"its init service.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			d, err := connect(cmd.Context(), cmd.OutOrStdout())
			if err != nil {
				return err
			}
			ctx, out := cmd.Context(), cmd.OutOrStdout()

			if yes || !prompt.IsTerminal() {
				fmt.Fprintf(out, "%s\n", styleTitle.Render("Installing on "+d.Serial()))
			} else {
				ok, err := prompt.ConfirmDefaultYes(ctx, out, "Install on "+d.Serial()+"?")
				if err != nil {
					return err
				}
				if !ok {
					return prompt.ErrCanceled
				}
			}

			if ok, err := d.IsRoot(); err == nil && !ok {
				fmt.Fprintf(
					out,
					"  %-18s %s\n",
					"adb root",
					styleDetail.Render("restarting adbd as root"),
				)
			}
			if err := d.Root(ctx); err != nil {
				return err
			}

			if mode, err := d.Shell(
				"getenforce",
			); err == nil &&
				strings.TrimSpace(mode) == "Enforcing" {
				if _, err := d.Shell("setenforce 0"); err != nil {
					return fmt.Errorf("selinux: %w", err)
				}
				fmt.Fprintf(out, "  %-18s %s\n", "selinux", styleDone.Render("permissive"))
			}

			checks, err := takeover.Preflight(d)
			if err != nil {
				return err
			}

			if blocked := takeover.BlockedExcept(checks, takeover.Verity); len(blocked) > 0 {
				fmt.Fprintf(out, "%s\n", styleFail.Render("the device is not ready"))
				for _, b := range blocked {
					fmt.Fprintf(out, "  %-18s %s  %s\n", b.Name, b.Got, styleDetail.Render(b.Fix))
				}
				return fmt.Errorf("%d prerequisite(s) not met", len(blocked))
			}

			if takeover.Failed(checks, takeover.Verity) {
				fmt.Fprintf(
					out,
					"  %-18s %s\n",
					"dm-verity",
					styleDetail.Render(
						"enforcing — disabling, the device will reboot and come back",
					),
				)

				rebooted, err := takeover.DisableVerity(ctx, d)
				if err != nil {
					return err
				}

				state := "disabled"
				if rebooted {
					state = "disabled, rebooted, root restored"
				}
				fmt.Fprintf(out, "  %-18s %s\n", "dm-verity", styleDone.Render(state))

				if checks, err = takeover.Preflight(d); err != nil {
					return err
				}
				if blocked := takeover.Blocked(checks); len(blocked) > 0 {
					return fmt.Errorf("%s is still %s", blocked[0].Name, blocked[0].Got)
				}
			}

			chosen, err := resolveName(ctx, out, d, name)
			if err != nil {
				return err
			}

			var patched bool
			if !noBoot {
				slots, err := takeover.Slots(d)
				if err != nil {
					return err
				}
				for _, slot := range slots {
					changed, err := takeover.MakePermissive(d, slot)
					if err != nil {
						return err
					}
					patched = patched || changed

					state := "already permissive"
					if changed {
						state = styleDone.Render("patched")
					}
					fmt.Fprintf(out, "  boot%-13s %s\n", slot, state)
				}
			}

			if patched {
				fmt.Fprintf(
					out,
					"  %-18s %s\n",
					"rebooting",
					styleDetail.Render("for the permissive boot command line"),
				)
				if err := d.Reboot(""); err != nil {
					return err
				}
				if err := d.WaitBooted(ctx); err != nil {
					return err
				}
				if err := d.Root(ctx); err != nil {
					return err
				}
				mode, _ := d.Shell("getenforce")
				if mode = strings.TrimSpace(mode); mode != "Permissive" {
					return fmt.Errorf("selinux is %s after the boot patch, want Permissive", mode)
				}
				fmt.Fprintf(
					out,
					"  %-18s %s\n",
					"selinux",
					styleDone.Render("permissive from the boot command line"),
				)
			}

			payload, from, err := lanovod(binary)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "  %-18s %s\n", "lanovod", styleDetail.Render(from))

			moved, err := takeover.Stash(d)
			for _, s := range moved {
				fmt.Fprintf(out, "  %-18s %s\n", s.What, styleDetail.Render(s.Note))
			}
			if err != nil {
				return err
			}

			room, err := takeover.Room(d, int64(len(payload)+len(takeover.InitRC)))
			if err != nil {
				return err
			}
			if !room.OK {
				fmt.Fprintf(out, "  %-18s %s\n", room.Name, styleFail.Render(room.Got))
				return fmt.Errorf("installing needs %s and / has %s — %s",
					room.Want, room.Got, room.Fix)
			}
			fmt.Fprintf(out, "  %-18s %s\n", room.Name, styleDetail.Render(room.Got))

			res, err := takeover.Install(d, payload)
			for _, s := range res.Steps {
				fmt.Fprintf(out, "  %-18s %s\n", s.What, styleDetail.Render(s.Note))
			}
			if err != nil {
				return err
			}

			if err := takeover.WriteName(d, chosen); err != nil {
				return err
			}
			fmt.Fprintf(out, "  %-18s %s\n", "name", styleDone.Render(chosen))

			settles := res.Settles
			choice := rebootChoiceOf(doReboot, noReboot)

			if settles && choice != rebootNo {
				fmt.Fprintf(out, "  %-18s %s\n", "rebooting",
					styleDetail.Render("for the service definition"))

				if err := d.Reboot(""); err != nil {
					return err
				}
				if err := takeover.WaitRunning(ctx, d); err != nil {
					return err
				}
				if err := d.Root(ctx); err != nil {
					return err
				}
				fmt.Fprintf(
					out,
					"  %-18s %s\n",
					"up",
					styleDone.Render("lanovod running, root restored"),
				)
				settles = false
			}

			if err := ensureWifi(
				ctx,
				out,
				d,
				wifisetup.Request{SSID: ssid, Password: password, WPS: wps},
			); err != nil {
				return err
			}

			return finish(ctx, out, d, settles, choice, genKey)
		},
	}

	c.Flags().StringVar(&binary, "binary", "", "install this lanovod rather than the one built in")
	nameFlag(c, &name)
	c.Flags().BoolVar(&yes, "yes", false, "do not ask before installing")
	c.Flags().BoolVar(&noBoot, "no-boot-patch", false, "leave the boot command line alone")
	c.Flags().StringVar(&ssid, "ssid", "", "network to join, instead of picking from a scan")
	c.Flags().StringVar(&password, "password", "", "passphrase, for an --ssid that needs one")
	c.Flags().BoolVar(&wps, "wps", false, "join by pressing the router's WPS button instead")
	c.Flags().BoolVar(&doReboot, "reboot", false, "reboot when something needs it, without asking")
	c.Flags().BoolVar(&noReboot, "no-reboot", false, "never reboot")
	c.Flags().BoolVar(&genKey, "generate-key", false,
		"set an encryption key now, instead of letting Home Assistant set one when it adds the device")
	return c
}

type rebootChoice int

const (
	rebootAsk rebootChoice = iota
	rebootYes
	rebootNo
)

func rebootChoiceOf(yes, no bool) rebootChoice {
	switch {
	case no:
		return rebootNo
	case yes:
		return rebootYes
	}
	return rebootAsk
}

func finish(
	ctx context.Context,
	out io.Writer,
	d *adb.Device,
	settles bool,
	choice rebootChoice,
	genKey bool,
) error {
	if err := takeover.Start(d); err != nil {
		return err
	}
	fmt.Fprintf(out, "  %-18s %s\n", "started", styleDone.Render("lanovod running"))

	if !settles || choice == rebootNo {
		reportKey(ctx, out, d, genKey)
		return nil
	}

	if choice == rebootAsk {
		if !prompt.IsTerminal() {
			fmt.Fprintf(out, "%s\n", styleDetail.Render(
				"some of this only takes effect on the next boot; pass --reboot to do it here"))
			return nil
		}
		yes, err := prompt.Confirm(
			ctx,
			out,
			"Reboot now? Some of this only takes effect on the next boot.",
		)
		if err != nil && !errors.Is(err, prompt.ErrCanceled) {
			return err
		}
		if !yes {
			reportKey(ctx, out, d, genKey)
			return nil
		}
	}

	if err := d.Reboot(""); err != nil {
		return err
	}
	fmt.Fprintf(out, "  %-18s %s\n", "rebooting", styleDetail.Render("waiting for it to come back"))
	if err := takeover.WaitRunning(ctx, d); err != nil {
		return err
	}
	fmt.Fprintf(out, "  %-18s %s\n", "up", styleDone.Render("lanovod started"))

	reportKey(ctx, out, d, genKey)
	return nil
}
