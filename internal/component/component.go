package component

import (
	"github.com/ygelfand/libcountertop/pkg/hook"
	sharedlib "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/config"
)

type Phase = sharedlib.Phase

const (
	Hardware = sharedlib.Hardware
	Device   = sharedlib.Device
	Network  = sharedlib.Network
)

type Component = sharedlib.Component
type Starter = sharedlib.Starter
type Closer = sharedlib.Closer
type Runner = sharedlib.Runner
type Entities = sharedlib.Entities
type Actions = sharedlib.Actions
type Handler = sharedlib.Handler
type Progress = sharedlib.Progress
type Startup = sharedlib.Startup
type Restorer = sharedlib.Restorer[config.Config]
type Event = sharedlib.Event

var Reconnect hook.Hook[struct{}]
var Fire hook.Hook[Event]
var Subscribed hook.Hook[struct{}]
