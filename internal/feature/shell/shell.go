// Package shell binds the shared screen stack to the product component registry.
package shell

import (
	"github.com/ygelfand/LANovo/internal/component"
	sharedlib "github.com/ygelfand/libcountertop/pkg/display/shell"
)

type View = sharedlib.View
type Timeouter = sharedlib.Timeouter
type Sleeper = sharedlib.Sleeper
type Waker = sharedlib.Waker
type Shell = sharedlib.Shell
type Hold = sharedlib.Hold
type Change = sharedlib.Change
type Stacked = sharedlib.Stacked

const DockTimeout = sharedlib.DockTimeout
const SettingsTimeout = sharedlib.SettingsTimeout

var Showing = sharedlib.Showing
var shared = sharedlib.New()

func Get() *Shell { return shared }
func init()       { component.Register(component.Device, Get, component.Order(34)) }
