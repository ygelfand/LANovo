package component

import "fmt"

// Sub-devices Home Assistant shows beneath this one, which entities join by Base.DeviceID. Zero is
// the device itself and is what most things want.
//
// The point is a shorter entity list per page, not grouping for its own sake: Home Assistant puts
// every entity of a device on one page, and there are enough of them here to be unreadable. Each of
// these is a real entry in the device registry, linked back to the device, so one is worth having
// only where it holds enough entities to be worth clicking into.
//
// Names are composed with the device's own at runtime. Home Assistant uses what it is given
// verbatim, so a bare "Playback" would be a device called "Playback" in a house with several of
// these.
//
// The numbers are identity: Home Assistant keys a sub-device on the address and this, so changing
// one orphans the entry it used to name. Add above DeviceAssistant, which has to stay last because
// the slots count upwards from it.
const (
	// DeviceScreen is the panel: how bright, which theme, which way the drawer comes in.
	DeviceScreen uint32 = iota + 1

	// DeviceMicrophone is the array: how it is combined, how hard it is driven, whether it is cut.
	DeviceMicrophone

	// DevicePlayback is how sound comes out, which is not the media player itself — that is the
	// thing people reach for, and it stays where it is found.
	DevicePlayback

	// DeviceCamera is the camera: the picture itself, and the exposure, tone, white balance and
	// orientation that shape it.
	DeviceCamera

	// DeviceAssistant is the first of one per wake word slot. Home Assistant keeps its own Assistant
	// and Wake word selects on the device itself, so choosing a wake word and tuning it are on
	// different pages.
	DeviceAssistant
)

// Assistants is how many wake word slots there are, and so how many assistant pages. Home
// Assistant's own interface offers two, and each is paired with its own pipeline.
const Assistants = 2

// AssistantDevice is the sub-device holding one slot's settings.
func AssistantDevice(slot int) uint32 { return DeviceAssistant + uint32(slot) }

// SubDevice is one page to advertise. Name is a suffix: Home Assistant shows what it is given
// verbatim, so the caller puts the device's own name in front of it.
type SubDevice struct {
	ID   uint32
	Name string
}

// SubDevices is the pages worth advertising, which is not every number above: a sub-device with no
// entities is an entry in the device registry with nothing to click into.
func SubDevices() []SubDevice {
	out := []SubDevice{
		{ID: DeviceScreen, Name: "screen"},
		{ID: DeviceMicrophone, Name: "microphone"},
		{ID: DevicePlayback, Name: "playback"},
		{ID: DeviceCamera, Name: "camera"},
	}

	for slot := range Assistants {
		out = append(out, SubDevice{
			ID:   AssistantDevice(slot),
			Name: fmt.Sprintf("assistant %d", slot+1),
		})
	}
	return out
}
