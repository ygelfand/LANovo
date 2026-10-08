package logging

import (
	"github.com/ygelfand/libcountertop/pkg/android/logd"

	"github.com/ygelfand/LANovo/internal/layout"
)

var Log = logd.NewLog(layout.LogTag)
