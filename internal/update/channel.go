package update

type Channel int

const (
	// GitHub's "latest" excludes prereleases.
	Stable Channel = iota

	Dev
)

const releases = "https://github.com/ygelfand/LANovo/releases"

func (c Channel) Label() string {
	if c == Dev {
		return "dev"
	}
	return "stable"
}

// api.github.com allows sixty unauthenticated requests an hour per address.
func (c Channel) URL() string {
	if c == Dev {
		return releases + "/download/dev/manifest.json"
	}
	return releases + "/latest/download/manifest.json"
}

func Channels() []Channel { return []Channel{Stable, Dev} }
