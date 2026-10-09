package component

import "fmt"

// Home Assistant keys a sub-device on the address and this number; renumbering orphans its entry.
const (
	DeviceScreen uint32 = iota + 1

	DeviceMicrophone

	DevicePlayback

	DeviceCamera

	DeviceAssistant
)

const (
	DeviceDashboard uint32 = 16 + iota
	DeviceLandscape
	DevicePortrait
)

var Sometimes = map[uint32]bool{DeviceDashboard: true, DeviceLandscape: true, DevicePortrait: true}

// Home Assistant's interface offers two wake word slots.
const Assistants = 2

func AssistantDevice(slot int) uint32 { return DeviceAssistant + uint32(slot) }

// Home Assistant shows the name verbatim.
type SubDevice struct {
	ID   uint32
	Name string
}

func SubDevices() []SubDevice {
	out := []SubDevice{
		{ID: DeviceScreen, Name: "screen"},
		{ID: DeviceMicrophone, Name: "microphone"},
		{ID: DevicePlayback, Name: "playback"},
		{ID: DeviceCamera, Name: "camera"},
		{ID: DeviceDashboard, Name: "dashboard"},
		{ID: DeviceLandscape, Name: "landscape"},
		{ID: DevicePortrait, Name: "portrait"},
	}

	for slot := range Assistants {
		out = append(out, SubDevice{
			ID:   AssistantDevice(slot),
			Name: fmt.Sprintf("assistant %d", slot+1),
		})
	}
	return out
}
