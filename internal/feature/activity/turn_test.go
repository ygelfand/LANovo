package activity

import (
	"testing"

	"github.com/ygelfand/libcountertop/pkg/assistant/activity"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
)

func TestAnEndedTurnIsFiredToHomeAssistant(t *testing.T) {
	var got []sharedcomponent.Event
	cancel := component.Fire.Listen(func(e sharedcomponent.Event) { got = append(got, e) })
	t.Cleanup(cancel)

	Get().Begin(1, "hey jarvis").Ends(activity.Completed)

	if len(got) != 1 || got[0].Name != TurnEvent {
		t.Fatalf("fired %v, want one %s", got, TurnEvent)
	}
	if got[0].Data["outcome"] != string(activity.Completed) {
		t.Errorf("outcome is %q, want %q", got[0].Data["outcome"], activity.Completed)
	}
}
