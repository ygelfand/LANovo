package shell

import (
	sharedshell "github.com/ygelfand/libcountertop/pkg/display/shell"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
)

var shell = sharedshell.New()

func Get() *sharedshell.Shell { return shell }

func init() { component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(34)) }
