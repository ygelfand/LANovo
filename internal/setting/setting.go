// Package setting binds shared settings descriptions to the product's translations.
package setting

import (
	"github.com/ygelfand/LANovo/internal/lib/say"
	shared "github.com/ygelfand/libcountertop/pkg/settings"
)

type Kind = shared.Kind
type Group = shared.Group
type Scale = shared.Scale
type Option = shared.Option
type Setting[T any] = shared.Setting[T]
type Table[T any] = shared.Table[T]
type Controls[T any] = shared.Controls[T]

const (
	On       = shared.On
	Off      = shared.Off
	Toggle   = shared.Toggle
	Number   = shared.Number
	Choice   = shared.Choice
	Pair     = shared.Pair
	Triple   = shared.Triple
	List     = shared.List
	Linear   = shared.Linear
	Doubling = shared.Doubling
)

var OnOff = shared.OnOff
var Boolean = shared.Boolean

func NewTable[T any](domain string, groups []Group, rows []Setting[T]) *Table[T] {
	return shared.NewTable(domain, groups, rows, shared.Messages{Text: say.T, Missing: say.Missing})
}
