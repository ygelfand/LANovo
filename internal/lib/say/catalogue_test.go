package say_test

import (
	"testing"

	"github.com/ygelfand/libcountertop/pkg/say/saytest"
)

func TestEveryIdentifierTheCodeAsksForExists(t *testing.T) { saytest.Catalogue(t, "../../..") }
