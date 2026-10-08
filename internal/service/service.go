package service

import shared "github.com/ygelfand/libcountertop/pkg/runtime/service"

type Service = shared.Service
type Starter = shared.Starter
type Closer = shared.Closer
type Reacquirer = shared.Reacquirer
type State = shared.State
type Status = shared.Status
type Option = shared.Option
type Group = shared.Group

const (
	StateWaiting  = shared.StateWaiting
	StateRunning  = shared.StateRunning
	StateRetrying = shared.StateRetrying
	StateFailed   = shared.StateFailed
	StateStopped  = shared.StateStopped
)

var New = shared.New
var Healthy = shared.Healthy
var Required = shared.Required
var Restart = shared.Restart
var Once = shared.Once
