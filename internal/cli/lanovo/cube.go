package lanovo

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"github.com/ygelfand/libcountertop/pkg/host/adb"
	"github.com/ygelfand/libcountertop/pkg/host/fastboot"
	"github.com/ygelfand/libcountertop/pkg/host/prompt"

	"github.com/ygelfand/LANovo/internal/host/takeover"
)

func cubeTakeover(ctx context.Context, out io.Writer, name, bootImage string, yes bool) error {
	if name != "cube" {
		return nil
	}

	clock, err := enterRecovery(ctx, out, bootImage, yes)
	if err != nil {
		return err
	}
	serial = clock.Serial()
	fmt.Fprintf(out, "  %-18s %s\n", "recovery", styleDone.Render(clock.Serial()+" up"))

	res, err := takeover.PrepareRecovery(clock)
	for _, s := range res.Steps {
		fmt.Fprintf(out, "  %-18s %s\n", s.What, styleDetail.Render(s.Note))
	}
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "  %-18s %s\n", "rebooting", styleDetail.Render("into Android"))
	if err := clock.Reboot(""); err != nil {
		return err
	}
	if err := clock.Wait(ctx, adb.StateOnline); err != nil {
		return err
	}
	fmt.Fprintf(out, "  %-18s %s\n", "android", styleDone.Render("up with adb"))
	return nil
}

func enterRecovery(
	ctx context.Context,
	out io.Writer,
	bootImage string,
	yes bool,
) (*adb.Device, error) {
	fmt.Fprintf(
		out,
		"  %s\n",
		styleDetail.Render("hold volume up and plug in the clock to enter fastboot"),
	)
	fb, err := fastboot.WaitForDevice(ctx, serial)
	if err != nil {
		return nil, err
	}
	clock := fb.Serial()

	checks, err := takeover.FastbootChecks(fb)
	if err != nil {
		return nil, err
	}
	if blocked := takeover.BlockedExcept(checks); len(blocked) > 0 {
		for _, c := range blocked {
			fmt.Fprintf(out, "  %-18s %s  %s\n", c.Name, c.Got, styleDetail.Render(c.Fix))
		}
		return nil, fmt.Errorf("the bootloader is not ready")
	}

	if err := confirmCube(ctx, out, clock, yes, "Flash %s?"); err != nil {
		return nil, err
	}

	image, from, err := resolveBootImage(ctx, out, bootImage)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(out, "  %-18s %s\n", "boot image", styleDetail.Render(from))

	part, err := takeover.FlashRecovery(ctx, fb, image)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(
		out,
		"  %-18s %s\n",
		"flashed",
		styleDone.Render(clock+" "+part+", rebooting to recovery"),
	)

	return waitForRecovery(ctx, clock)
}

func confirmCube(ctx context.Context, out io.Writer, serial string, yes bool, format string) error {
	msg := fmt.Sprintf(format, serial)
	if yes || !prompt.IsTerminal() {
		fmt.Fprintf(out, "%s\n", styleTitle.Render(msg))
		return nil
	}
	ok, err := prompt.Confirm(ctx, out, msg)
	if err != nil {
		return err
	}
	if !ok {
		return prompt.ErrCanceled
	}
	return nil
}

func resolveBootImage(ctx context.Context, out io.Writer, path string) ([]byte, string, error) {
	img, ok := takeover.BootFor("cube")
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, "", err
		}
		if ok {
			if err := img.Verify(path, data); err != nil {
				return nil, "", err
			}
		}
		return data, path, nil
	}
	if !ok {
		return nil, "", fmt.Errorf(
			"no boot image is published for cube; give one with --boot-image",
		)
	}

	title := fmt.Sprintf("Fetching the cube boot image (%.1f MB)", float64(img.Size)/(1<<20))
	return fetchImage(ctx, out, title, img.Resolve)
}

func fetchImage(ctx context.Context, out io.Writer, title string,
	resolve func(context.Context, func(float64)) ([]byte, string, error),
) ([]byte, string, error) {
	if !prompt.IsTerminal() {
		fmt.Fprintln(out, title)
		var last int
		return resolve(ctx, func(f float64) {
			if pct := int(f * 100); pct >= last+10 {
				last = pct
				fmt.Fprintf(out, "  %d%%\n", pct)
			}
		})
	}

	m := &fetchModel{
		title: title,
		bar:   progress.New(progress.WithDefaultBlend(), progress.WithWidth(barWidth)),
	}
	prog := tea.NewProgram(m, tea.WithContext(ctx), tea.WithOutput(out))

	go func() {
		data, from, err := resolve(ctx, func(f float64) { prog.Send(fetchAt(f)) })
		prog.Send(fetchDone{data: data, from: from, err: err})
	}()

	final, err := prog.Run()
	if err != nil {
		return nil, "", err
	}
	done := final.(*fetchModel)
	return done.data, done.from, done.err
}

type (
	fetchAt   float64
	fetchDone struct {
		data []byte
		from string
		err  error
	}
)

const barWidth = 40

type fetchModel struct {
	title string
	bar   progress.Model
	at    float64

	data []byte
	from string
	err  error
}

func (m *fetchModel) Init() tea.Cmd { return nil }

func (m *fetchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case fetchAt:
		m.at = float64(msg)
		return m, nil
	case fetchDone:
		m.data, m.from, m.err = msg.data, msg.from, msg.err
		return m, tea.Quit
	case tea.WindowSizeMsg:
		m.bar.SetWidth(min(barWidth, max(msg.Width-10, 10)))
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.err = prompt.ErrCanceled
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *fetchModel) View() tea.View {
	return tea.NewView(styleTitle.Render(m.title) + "\n\n  " + m.bar.ViewAs(m.at) + "\n")
}

func waitForRecovery(ctx context.Context, serial string) (*adb.Device, error) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		if d, err := adb.AttachAny(serial); err == nil {
			if rec, _ := d.InRecovery(); rec {
				return d, nil
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-tick.C:
		}
	}
}
