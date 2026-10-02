package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fake is a service that does what a test tells it to.
type fake struct {
	name string

	mu       sync.Mutex
	starts   int
	runs     int
	closes   int
	order    *[]string
	startErr error

	// run is what Run does. Returning an error looks like breaking.
	run func(ctx context.Context) error
}

func (f *fake) Name() string { return f.name }

func (f *fake) Start(ctx context.Context) error {
	f.mu.Lock()
	f.starts++
	if f.order != nil {
		*f.order = append(*f.order, "start:"+f.name)
	}
	err := f.startErr
	f.mu.Unlock()
	return err
}

func (f *fake) Run(ctx context.Context) error {
	f.mu.Lock()
	f.runs++
	run := f.run
	f.mu.Unlock()

	if run != nil {
		return run(ctx)
	}
	<-ctx.Done()
	return nil
}

func (f *fake) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.closes++
	if f.order != nil {
		*f.order = append(*f.order, "close:"+f.name)
	}
	return nil
}

func (f *fake) counts() (starts, runs, closes int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.starts, f.runs, f.closes
}

// blocks until canceled, which is what most services do.
func blocks(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Services come up in the order they were added and go down in reverse, because acquiring hardware
// off Android is order-dependent and releasing it should unwind.
func TestStartsInOrderAndClosesInReverse(t *testing.T) {
	var order []string
	a := &fake{name: "a", order: &order, run: blocks}
	b := &fake{name: "b", order: &order, run: blocks}
	c := &fake{name: "c", order: &order, run: blocks}

	g := New()
	g.Add(a)
	g.Add(b)
	g.Add(c)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- g.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := []string{"start:a", "start:b", "start:c", "close:c", "close:b", "close:a"}
	if len(order) != len(want) {
		t.Fatalf("order was %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order was %v, want %v", order, want)
		}
	}
}

// A required service that cannot be acquired is fatal, and nothing after it is started: the process
// is better off exiting and being restarted by init than limping.
func TestRequiredStartFailureIsFatal(t *testing.T) {
	broken := &fake{name: "broken", startErr: errors.New("no device")}
	after := &fake{name: "after", run: blocks}

	g := New()
	g.Add(broken, Required())
	g.Add(after)

	err := g.Run(context.Background())
	if err == nil {
		t.Fatal("Run returned nil for a failed required service")
	}

	if starts, _, _ := after.counts(); starts != 0 {
		t.Errorf("the service after a fatal one was started %d times", starts)
	}
	if got := statusOf(g, "broken").State; got != StateFailed {
		t.Errorf("state is %q, want failed", got)
	}
}

// An optional service that cannot be acquired is recorded and the rest carry on, which is how a
// device with no speaker still answers.
func TestOptionalStartFailureDegrades(t *testing.T) {
	broken := &fake{name: "broken", startErr: errors.New("no device")}
	after := &fake{name: "after", run: blocks}

	g := New()
	g.Add(broken)
	g.Add(after)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- g.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)
	if starts, _, _ := after.counts(); starts != 1 {
		t.Errorf("the service after an optional failure started %d times, want 1", starts)
	}
	if Healthy(g.Status()) {
		t.Error("the group reports healthy with a failed service")
	}

	cancel()
	<-done
}

// The whole point: a service that breaks is closed, acquired again and run again.
func TestRestartsAfterFailure(t *testing.T) {
	var attempts int
	var mu sync.Mutex

	svc := &fake{name: "flaky"}
	svc.run = func(ctx context.Context) error {
		mu.Lock()
		attempts++
		n := attempts
		mu.Unlock()

		if n < 3 {
			return errors.New("broke")
		}
		<-ctx.Done()
		return nil
	}

	g := New()
	g.Add(svc, Restart(5*time.Millisecond, 10*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- g.Run(ctx) }()

	deadline := time.Now().Add(2 * time.Second)
	for {
		if s := statusOf(g, "flaky"); s.State == StateRunning && s.Restarts >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("never settled: %+v", statusOf(g, "flaky"))
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Re-acquired each time, not just re-run: the device may have been taken while it was gone.
	starts, runs, closes := svc.counts()
	if starts < 3 || runs < 3 || closes < 2 {
		t.Errorf("starts=%d runs=%d closes=%d, want at least 3/3/2", starts, runs, closes)
	}

	cancel()
	<-done
}

// A service with no restart policy that breaks stays broken and says so.
func TestFailureWithoutRestartStaysFailed(t *testing.T) {
	svc := &fake{name: "once", run: func(context.Context) error { return errors.New("broke") }}

	g := New()
	g.Add(svc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = g.Run(ctx) }()

	deadline := time.Now().Add(time.Second)
	for statusOf(g, "once").State != StateFailed {
		if time.Now().After(deadline) {
			t.Fatalf("state is %q, want failed", statusOf(g, "once").State)
		}
		time.Sleep(5 * time.Millisecond)
	}

	if _, runs, _ := svc.counts(); runs != 1 {
		t.Errorf("ran %d times without a restart policy, want 1", runs)
	}
	if err := statusOf(g, "once").Err; err == nil {
		t.Error("no error recorded for a failed service")
	}
}

// A one-shot service returning nil is finished, not broken.
func TestOneShotStops(t *testing.T) {
	svc := &fake{name: "splash", run: func(context.Context) error { return nil }}

	g := New()
	g.Add(svc, Once())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = g.Run(ctx) }()

	deadline := time.Now().Add(time.Second)
	for statusOf(g, "splash").State != StateStopped {
		if time.Now().After(deadline) {
			t.Fatalf("state is %q, want stopped", statusOf(g, "splash").State)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !Healthy(g.Status()) {
		t.Error("a finished one-shot service makes the group unhealthy")
	}
}

// A service with no Start or Close is still supervised.
type bare struct{ name string }

func (b bare) Name() string                  { return b.name }
func (b bare) Run(ctx context.Context) error { <-ctx.Done(); return nil }

func TestServiceWithoutStartOrClose(t *testing.T) {
	g := New()
	g.Add(bare{name: "plain"})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- g.Run(ctx) }()

	time.Sleep(30 * time.Millisecond)
	if got := statusOf(g, "plain").State; got != StateRunning {
		t.Errorf("state is %q, want running", got)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}
}

// A service that cannot take its device back must not be run without it. Run would report itself
// up while holding nothing, and one that waits on hardware it never got never returns — so it is
// never restarted either, and the service is stuck for the life of the process.
func TestNotRunUntilReacquired(t *testing.T) {
	svc := &fake{name: "device"}
	svc.run = func(ctx context.Context) error {
		if _, runs, _ := svc.counts(); runs == 1 {
			// Break, and make the device unavailable from here on.
			svc.mu.Lock()
			svc.startErr = errors.New("device busy")
			svc.mu.Unlock()
			return errors.New("lost the device")
		}

		<-ctx.Done()
		return nil
	}

	g := New()
	g.Add(svc, Restart(time.Millisecond, 2*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- g.Run(ctx) }()

	// Wait for it to have tried, and failed, to re-acquire several times over.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if starts, _, _ := svc.counts(); starts >= 4 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("never retried: %+v", statusOf(g, "device"))
		}
		time.Sleep(time.Millisecond)
	}

	if _, runs, _ := svc.counts(); runs != 1 {
		t.Errorf("Run was called %d times while the device could not be acquired, want 1", runs)
	}
	if s := statusOf(g, "device"); s.State != StateRetrying {
		t.Errorf("state = %q, want %q while it cannot be acquired", s.State, StateRetrying)
	}

	cancel()
	<-done
}

// Once the device comes back the service runs again, rather than the retry loop being somewhere it
// cannot leave.
func TestRunsOnceReacquired(t *testing.T) {
	svc := &fake{name: "device"}
	svc.run = func(ctx context.Context) error {
		if _, runs, _ := svc.counts(); runs == 1 {
			svc.mu.Lock()
			svc.startErr = errors.New("device busy")
			svc.mu.Unlock()
			return errors.New("lost the device")
		}

		<-ctx.Done()
		return nil
	}

	g := New()
	g.Add(svc, Restart(time.Millisecond, 2*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- g.Run(ctx) }()

	// Give the device back part way through.
	giveBack := time.Now().Add(2 * time.Second)
	for {
		if starts, _, _ := svc.counts(); starts >= 3 {
			break
		}
		if time.Now().After(giveBack) {
			cancel()
			<-done
			t.Fatalf("never retried: %+v", statusOf(g, "device"))
		}
		time.Sleep(time.Millisecond)
	}
	svc.mu.Lock()
	svc.startErr = nil
	svc.mu.Unlock()

	deadline := time.Now().Add(2 * time.Second)
	for {
		if s := statusOf(g, "device"); s.State == StateRunning {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("never came back: %+v", statusOf(g, "device"))
		}
		time.Sleep(time.Millisecond)
	}

	if _, runs, _ := svc.counts(); runs != 2 {
		t.Errorf("Run was called %d times, want 2: once before losing the device and once after", runs)
	}

	cancel()
	<-done
}

func statusOf(g *Group, name string) Status {
	for _, s := range g.Status() {
		if s.Name == name {
			return s
		}
	}
	return Status{}
}

// keeper is a service that lets go of less on a restart than on a stop.
type keeper struct {
	fake

	releases int
}

func (k *keeper) Release() error {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.releases++
	if k.order != nil {
		*k.order = append(*k.order, "release:"+k.name)
	}
	return nil
}

func (k *keeper) letGo() (releases, closes int) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.releases, k.closes
}

// A restart calls Release, not Close. The DHCP client is the case: its Close hands the lease back
// and strips the address, so a restart that closed would put the device off the network between
// two runs of its own loop, and leave it there if the restart then failed.
func TestARestartReleasesRatherThanCloses(t *testing.T) {
	broke := make(chan struct{}, 1)
	svc := &keeper{fake: fake{name: "net", run: func(ctx context.Context) error {
		select {
		case broke <- struct{}{}:
			return errors.New("fell over")
		default:
			<-ctx.Done()
			return nil
		}
	}}}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g := New()
	g.Add(svc, Restart(time.Millisecond, 2*time.Millisecond))

	done := make(chan error, 1)
	go func() { done <- g.Run(ctx) }()

	// Wait for the restart to have happened: two runs means the first returned and the second began.
	waitFor(t, func() bool { _, runs, _ := svc.counts(); return runs >= 2 })

	if releases, closes := svc.letGo(); releases != 1 || closes != 0 {
		t.Errorf("after a restart: %d releases and %d closes, want one release and no close", releases, closes)
	}

	cancel()
	<-done

	// And stopping still closes, because that is when giving the address back is the right thing.
	if _, closes := svc.letGo(); closes != 1 {
		t.Errorf("after stopping: %d closes, want one", closes)
	}
}

// A service that says nothing about the difference is closed on a restart, which is the right
// default: letting go of a device is usually the whole point of restarting.
func TestAServiceWithNoReleaseIsStillClosedOnARestart(t *testing.T) {
	broke := make(chan struct{}, 1)
	svc := &fake{name: "card", run: func(ctx context.Context) error {
		select {
		case broke <- struct{}{}:
			return errors.New("fell over")
		default:
			<-ctx.Done()
			return nil
		}
	}}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g := New()
	g.Add(svc, Restart(time.Millisecond, 2*time.Millisecond))

	done := make(chan error, 1)
	go func() { done <- g.Run(ctx) }()

	waitFor(t, func() bool { _, runs, _ := svc.counts(); return runs >= 2 })

	if _, _, closes := svc.counts(); closes != 1 {
		t.Errorf("%d closes across a restart, want one", closes)
	}

	cancel()
	<-done
}

// waitFor blocks until want is true, or fails the test.
func waitFor(t *testing.T, want func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if want() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out")
}
