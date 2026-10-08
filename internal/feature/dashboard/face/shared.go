package face

import (
	shared "github.com/ygelfand/libcountertop/pkg/display/clockface"

	"github.com/ygelfand/LANovo/internal/config"
)

type Face = shared.Face
type Ticking = shared.Ticking
type Reading = shared.Reading

var Read = shared.Read
var Say = shared.Say
var Lit = shared.Lit
var Bars = shared.Bars

func Of(name config.Face) Face    { return shared.Of(shared.Kind(name)) }
func Ticks(name config.Face) bool { return shared.Ticks(shared.Kind(name)) }
