package obex

// Cover art, which is the Basic Imaging profile with AVRCP's own target uuid on the front.

// CoverArtTarget is the uuid a connect names to reach the image service. It is AVRCP's own rather
// than one of the imaging profile's, because this is cover art rather than browsing a camera.
var CoverArtTarget = []byte{
	0x71, 0x63, 0xdd, 0x54, 0x4a, 0x7e, 0x11, 0xe2,
	0xb4, 0x7c, 0x00, 0x50, 0xc2, 0x49, 0x00, 0x48,
}

// TypeThumbnail is what a get asks for, as the type header's value. Null terminated, as the profile
// writes it.
//
// The one form there is worth asking for: the profile fixes a thumbnail at 200x200 jpeg and every
// target must produce it, and the targets tried hold nothing else.
const TypeThumbnail = "x-bt/img-thm"

// HeaderImgHandle is where a target reads which image is wanted.
const HeaderImgHandle = 0x30

// Thumbnail asks for the artwork at the fixed size every target can produce.
func Thumbnail(connection uint32, handle string) Packet {
	return Get(connection,
		Bytes(HeaderType, terminated(TypeThumbnail)),
		Text(HeaderImgHandle, handle),
	)
}

// terminated is a type header's value: ascii with a trailing zero, which the profile requires and
// which a target checks for.
func terminated(s string) []byte { return append([]byte(s), 0) }
