package media

import (
	sharedcard "github.com/ygelfand/libcountertop/pkg/display/mediacard"
	sharedshell "github.com/ygelfand/libcountertop/pkg/display/shell"

	"github.com/ygelfand/LANovo/internal/feature/shell"
)

var theCard *sharedcard.Card

func card() *sharedcard.Card { Get(); return theCard }
func Page() sharedshell.View { return card().Page() }
func Showing() bool          { return shell.Get().Top() == Page() }
