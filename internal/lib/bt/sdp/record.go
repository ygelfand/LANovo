package sdp

import "sort"

// A service record is the answer to "what is this device". For A2DP it needs to say four things:
// there is an audio sink here, it speaks AVDTP, this is the channel to reach it on, and it conforms
// to the Advanced Audio Distribution profile at a version the phone recognises.
//
// Leave any of them out and the device either does not appear or appears and cannot be connected
// to, which is a worse failure because it looks like the audio is broken.

// The assigned numbers this record is made of. All from the Bluetooth SIG's list, which is why they
// are short UUIDs rather than anything of ours.
const (
	UUIDL2CAP      = 0x0100
	UUIDAVDTP      = 0x0019
	UUIDAVCTP      = 0x0017
	UUIDOBEX       = 0x0008
	UUIDAudioSink  = 0x110b
	UUIDA2DP       = 0x110d
	UUIDBrowseRoot = 0x1002

	UUIDAVRemoteControlTarget     = 0x110c
	UUIDAVRemoteControl           = 0x110e
	UUIDAVRemoteControlController = 0x110f
)

// The attribute identifiers. Everything below 0x0100 is defined for all services; above it is the
// profile's own.
const (
	AttrRecordHandle   = 0x0000
	AttrServiceClasses = 0x0001
	AttrProtocols      = 0x0004
	AttrBrowseGroups   = 0x0005
	AttrProfiles       = 0x0009
	AttrMoreProtocols  = 0x000d
	AttrName           = 0x0100
	AttrFeatures       = 0x0311
)

// What an audio sink can be. A phone shows a different icon for a headset than for a speaker, so
// this is not cosmetic.
const (
	FeatureHeadphone = 1 << 0
	FeatureSpeaker   = 1 << 1
	FeatureRecorder  = 1 << 2
	FeatureAmplifier = 1 << 3
)

// A2DPVersion is the profile version this record claims. 1.3 is what every phone since about 2012
// speaks and what SBC alone satisfies.
const A2DPVersion = 0x0103

// AVDTPVersion is the transport version, which moves independently of the profile's.
const AVDTPVersion = 0x0103

// Attribute is one field of a record.
type Attribute struct {
	ID    uint16
	Value Element
}

// Record is a service, as a set of attributes.
type Record []Attribute

// Attribute is the one with this id, and whether it is there.
func (r Record) Attribute(id uint16) (Element, bool) {
	for _, a := range r {
		if a.ID == id {
			return a.Value, true
		}
	}
	return Element{}, false
}

// Element is the record as it goes on the wire: one sequence of alternating identifiers and values.
//
// Sorted by identifier, which the spec requires — a phone reading a record is allowed to stop
// looking once it passes the one it wants.
func (r Record) Element() Element {
	sorted := make(Record, len(r))
	copy(sorted, r)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	// Two bytes each, always. An identifier is a fixed width in the spec, and one written narrow
	// because it happened to be small is a list a phone reads as something else entirely.
	out := make([]Element, 0, len(sorted)*2)
	for _, a := range sorted {
		out = append(out, Uint16(a.ID), a.Value)
	}
	return Sequence(out...)
}

// AudioSink is the record for this device as something a phone can play to.
//
// psm is the L2CAP channel AVDTP is served on, which is 0x0019 unless there is a reason.
func AudioSink(handle uint32, name string, psm uint16, features uint16) Record {
	return Record{
		{ID: AttrRecordHandle, Value: Uint32(handle)},

		// What it is. One class: an audio sink.
		{ID: AttrServiceClasses, Value: Sequence(UUID16(UUIDAudioSink))},

		// How to reach it, innermost protocol last. L2CAP carries the channel number as a
		// parameter, which is the part a phone actually uses to connect.
		{ID: AttrProtocols, Value: Sequence(
			Sequence(UUID16(UUIDL2CAP), Uint16(psm)),
			Sequence(UUID16(UUIDAVDTP), Uint16(AVDTPVersion)),
		)},

		// Which browse group it appears in. The public root is the one a phone looks in, and a
		// record left out of it exists but is never found.
		{ID: AttrBrowseGroups, Value: Sequence(UUID16(UUIDBrowseRoot))},

		// Which profile and version, which is what a phone matches against to decide it knows how
		// to talk to this.
		{ID: AttrProfiles, Value: Sequence(
			Sequence(UUID16(UUIDA2DP), Uint16(A2DPVersion)),
		)},

		{ID: AttrName, Value: Text(name)},
		{ID: AttrFeatures, Value: Uint16(features)},
	}
}

// UUIDs is every UUID anywhere in a record, which is what a search pattern matches against.
//
// Not the service class list. The spec matches a pattern against any UUID the record contains, at
// any depth, and phones rely on that: browsing is a search for the public browse group's UUID, and
// that one lives in the browse group list rather than among the classes. Matching on the classes
// alone gives a device that answers when asked for by name and is invisible to anything that goes
// looking.
func (r Record) UUIDs() []uint32 {
	var out []uint32
	for _, a := range r {
		collectUUIDs(a.Value, &out)
	}
	return out
}

func collectUUIDs(e Element, out *[]uint32) {
	if e.Type == TypeUUID {
		if v, ok := e.Uint(); ok {
			*out = append(*out, v)
		}
		return
	}
	for _, c := range e.Children {
		collectUUIDs(c, out)
	}
}

// AVRCPVersion is the remote control profile version these records claim. 1.6 is what gets a phone
// to send metadata and absolute volume; older ones get buttons and nothing else.
const AVRCPVersion = 0x0106

// AVCTPVersion is its transport's, which moves independently.
const AVCTPVersion = 0x0104

// What a remote control end can be, as the feature bits say it.
//
// These are the AV/C device categories, and a phone reads them to decide what to offer. Category 1
// is something that plays; category 2 is something that is loud. This device is both: it sends
// transport commands like a player's remote, and it takes absolute volume like an amplifier.
const (
	CategoryPlayer    = 1 << 0
	CategoryAmplifier = 1 << 1
	CategoryTuner     = 1 << 2
	CategoryMenu      = 1 << 3
)

// FeatureBrowsing says this end speaks the browsing channel.
//
// A phone reads it to decide whether the other end is worth the multi-player half of the profile:
// which players exist, which one is addressed, and the events that say either has changed. It says
// only that, though — the channel's own number lives in AttrMoreProtocols, and a record carrying
// the bit without it names a capability with no address to reach it at.
const FeatureBrowsing = 1 << 6

// What a controller will fetch over the cover art channel, which is BIP over OBEX on a channel the
// target publishes in AttrMoreProtocols.
//
// A controller's record only. A target's uses these same numbers for other things — 0x0080 is
// multiple players there and 0x0100 is cover art served rather than wanted — so setting them on
// both ends says this device serves images, and a phone that believes it goes looking for a
// channel that is not published.
const (
	FeatureImageProperties = 1 << 7
	FeatureImage           = 1 << 8
	FeatureThumbnail       = 1 << 9
)

// RemoteControl is a record for one end of AVRCP.
//
// Two of them get advertised, under different classes on the same channel: the target, which is
// what a phone talks to when it wants to set the volume, and the controller, which is what says
// this device may send it play and pause. A device advertising only one gets half the profile and
// no obvious sign of why.
// browse is the channel the browsing pdus are served on, or zero for an end that does not serve
// them. A non-zero one both sets the feature bit and publishes the number, which have to travel
// together: the bit alone is a capability with no address to reach it at.
func RemoteControl(handle uint32, name string, psm, browse uint16, class uint16, features uint16) Record {
	classes := []Element{UUID16(class)}

	// Both ends also carry the generic remote control class. Phones look for either, and the ones
	// that look for this one find nothing without it.
	if class != UUIDAVRemoteControl {
		classes = append(classes, UUID16(UUIDAVRemoteControl))
	}

	if browse != 0 {
		features |= FeatureBrowsing
	}

	r := Record{
		{ID: AttrRecordHandle, Value: Uint32(handle)},
		{ID: AttrServiceClasses, Value: Sequence(classes...)},

		{ID: AttrProtocols, Value: Sequence(
			Sequence(UUID16(UUIDL2CAP), Uint16(psm)),
			Sequence(UUID16(UUIDAVCTP), Uint16(AVCTPVersion)),
		)},

		{ID: AttrBrowseGroups, Value: Sequence(UUID16(UUIDBrowseRoot))},

		// The profile is the generic one whichever end this is. The class above is what says which
		// end, and the version here is what decides whether metadata and absolute volume are on
		// the table at all.
		{ID: AttrProfiles, Value: Sequence(
			Sequence(UUID16(UUIDAVRemoteControl), Uint16(AVRCPVersion)),
		)},

		{ID: AttrName, Value: Text(name)},
		{ID: AttrFeatures, Value: Uint16(features)},
	}

	if browse == 0 {
		return r
	}

	// A list of protocol descriptor lists, so one level deeper than AttrProtocols. Written flat, a
	// phone reads the browsing channel's number as a second protocol on the control channel.
	return append(r, Attribute{ID: AttrMoreProtocols, Value: Sequence(
		Sequence(
			Sequence(UUID16(UUIDL2CAP), Uint16(browse)),
			Sequence(UUID16(UUIDAVCTP), Uint16(AVCTPVersion)),
		),
	)})
}

// PSM reads the L2CAP channel out of a protocol descriptor list, which is what a phone does with
// this record and therefore the thing worth being able to check.
func PSM(protocols Element) (uint16, bool) {
	if protocols.Type != TypeSequence {
		return 0, false
	}

	for _, layer := range protocols.Children {
		if layer.Type != TypeSequence || len(layer.Children) < 2 {
			continue
		}

		id, ok := layer.Children[0].Uint()
		if !ok || id != UUIDL2CAP {
			continue
		}
		if psm, ok := layer.Children[1].Uint(); ok {
			return uint16(psm), true
		}
	}
	return 0, false
}
