package update

// Channel is which stream of releases a device follows. The URLs are compiled in, so Home Assistant
// can move a device between our streams but cannot aim it at somebody else's binary.
type Channel int

const (
	// Stable is the default. GitHub's "latest" excludes prereleases, so a stable device never sees a
	// dev build.
	Stable Channel = iota

	// Dev is the rolling release whose assets are replaced by hand.
	Dev
)

const releases = "https://github.com/ygelfand/LANovo/releases"

func (c Channel) Label() string {
	if c == Dev {
		return "dev"
	}
	return "stable"
}

// URL is where that channel's manifest lives. Asset downloads, not api.github.com: the REST API
// allows sixty unauthenticated requests an hour per address.
func (c Channel) URL() string {
	if c == Dev {
		return releases + "/download/dev/manifest.json"
	}
	return releases + "/latest/download/manifest.json"
}

// Channels is every channel, in the order Home Assistant offers them.
func Channels() []Channel { return []Channel{Stable, Dev} }
