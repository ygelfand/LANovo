package control

import (
	"time"

	harness "github.com/ygelfand/libcountertop/pkg/runtime/control"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/metrics"
	"github.com/ygelfand/LANovo/internal/layout"
)

func deviceInfo() harness.DeviceInfo {
	d := config.Get().Device
	return harness.DeviceInfo{
		Version: layout.Version,
		Commit:  layout.GitCommit,
		Built:   layout.BuildDate,
		Board:   board.Current().Name,
		Name:    d.Name,
		Address: d.Addr,
		Model:   d.Model,
		Uptime:  time.Duration(metrics.Uptime() * float64(time.Second)),
	}
}
