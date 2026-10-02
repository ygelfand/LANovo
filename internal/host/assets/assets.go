// Package assets holds what lanovoctl ships inside itself.
//
// A release is one download with nothing to fetch and no paths to get wrong. lanovod is the one
// thing that has to be built, so a build without it staged is still buildable — the accessor comes
// back empty and the caller says what is missing.
package assets

// Lanovod is the arm binary installed to /system/bin/lanovod, or empty in a build without a
// payload.
func Lanovod() []byte { return lanovod }

// Embedded reports whether this build carries lanovod.
func Embedded() bool { return len(lanovod) > 0 }
