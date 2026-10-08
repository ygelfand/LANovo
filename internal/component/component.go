package component

import (
	"github.com/ygelfand/libcountertop/pkg/hook"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
)

var Reconnect hook.Hook[struct{}]
var Fire hook.Hook[sharedcomponent.Event]
var Subscribed hook.Hook[struct{}]
