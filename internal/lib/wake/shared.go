package wake

import (
	"sync"

	shared "github.com/ygelfand/libcountertop/pkg/inference/wake"

	"github.com/ygelfand/LANovo/internal/layout"
)

type Model = shared.Model
type Kind = shared.Kind
type Library = shared.Library

const (
	DefaultModel      = shared.DefaultModel
	WindowSize        = shared.WindowSize
	FeatureStep       = shared.FeatureStep
	KindOpenWakeWord  = shared.KindOpenWakeWord
	KindMicroWakeWord = shared.KindMicroWakeWord
)

var Installed = shared.Installed
var Pick = shared.Pick
var Find = shared.Find
var OfKind = shared.OfKind
var Adopt = shared.Adopt
var Have = shared.Have
var Cached = shared.Cached
var Purge = shared.Purge
var NewLibrary = shared.NewLibrary
var get = sync.OnceValue(func() *Library { return shared.NewLibrary(layout.ModelDir) })

func Lib() *Library { return get() }
