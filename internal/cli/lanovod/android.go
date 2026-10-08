package lanovod

import "github.com/ygelfand/LANovo/internal/android/prop"

func stopService(name string) error { return prop.Stop(prop.Local, name) }
