package lanovod

import "github.com/ygelfand/LANovo/internal/android/prop"

// stopService stops an Android service through init.
func stopService(name string) error { return prop.Stop(prop.Local, name) }
