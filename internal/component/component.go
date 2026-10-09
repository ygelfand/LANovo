package component

import (
	setting "github.com/ygelfand/libcountertop/pkg/settings"

	"github.com/ygelfand/libcountertop/pkg/hook"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
)

var Settings setting.Registry

var Reconnect hook.Hook[struct{}]
var Fire hook.Hook[sharedcomponent.Event]
var Subscribed hook.Hook[struct{}]
