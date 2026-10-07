package config

import "github.com/ygelfand/libcountertop/pkg/settings/schema"

type Edge = schema.Edge

const (
	EdgeLeft    = schema.EdgeLeft
	EdgeRight   = schema.EdgeRight
	EdgeTop     = schema.EdgeTop
	EdgeBottom  = schema.EdgeBottom
	DefaultEdge = schema.DefaultEdge
)

var Edges = schema.Edges
var Sides = schema.Sides
