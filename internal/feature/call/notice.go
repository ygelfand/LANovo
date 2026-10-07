package call

import (
	"github.com/ygelfand/LANovo/internal/feature/message"
	"github.com/ygelfand/libcountertop/pkg/say"
	"time"
)

const noticeFor = 4 * time.Second

func notice(peer string, reason Reason) {
	var body string
	switch reason {
	case ReasonDeclined:
		body = say.T("call.declined")
	case ReasonUnanswered:
		body = say.T("call.unanswered")
	case ReasonBusy:
		body = say.T("call.busy")
	case ReasonUnavailable:
		body = say.T("call.unavailable")
	case ReasonDropped:
		body = say.T("call.dropped")
	case ReasonFailed:
		body = say.T("call.failed")
	default:
		return
	}
	message.Get().Show(message.Message{Title: peer, Body: body, Tone: message.ToneInfo}, noticeFor)
}
