package theme

// All is every theme that can be chosen, in the order they are offered.
//
// Dark ones first: the device is a lamp in a room, and most of the time it should be the quieter
// thing in it.
var All = []Theme{
	{
		Name: "Midnight", Dark: true,
		Background: rgb(0x0b0e14), Surface: rgb(0x151a23),
		Text: rgb(0xe6edf3), Muted: rgb(0x8b97a8),
		Accent: rgb(0x4c9aff), Accent2: rgb(0x9d7cff),
		Success: rgb(0x3fb950), Warning: rgb(0xd29922), Danger: rgb(0xf85149),
	},
	{
		Name: "Nocturne", Dark: true,
		Background: rgb(0x11131a), Surface: rgb(0x1b1e28),
		Text: rgb(0xe8e6e3), Muted: rgb(0x9a97a3),
		Accent: rgb(0xc8a2ff), Accent2: rgb(0x7fd1e8),
		Success: rgb(0x8fd67a), Warning: rgb(0xe5b567), Danger: rgb(0xe86f6f),
	},
	{
		Name: "Slate", Dark: true,
		Background: rgb(0x14171c), Surface: rgb(0x1f242c),
		Text: rgb(0xdfe4ea), Muted: rgb(0x8a929e),
		Accent: rgb(0x5ec8c8), Accent2: rgb(0x8fa6c4),
		Success: rgb(0x6cc070), Warning: rgb(0xd8a657), Danger: rgb(0xe06c6c),
	},
	{
		Name: "Ember", Dark: true,
		Background: rgb(0x14100e), Surface: rgb(0x221a16),
		Text: rgb(0xf3e7dd), Muted: rgb(0xa8948a),
		Accent: rgb(0xff8c42), Accent2: rgb(0xffc15e),
		Success: rgb(0x9ac16b), Warning: rgb(0xe8a33d), Danger: rgb(0xe45b5b),
	},
	{
		Name: "Forest", Dark: true,
		Background: rgb(0x0e1512), Surface: rgb(0x16211c),
		Text: rgb(0xdfeae2), Muted: rgb(0x85998c),
		Accent: rgb(0x5fbf8f), Accent2: rgb(0xa3d977),
		Success: rgb(0x63c97f), Warning: rgb(0xd4a95c), Danger: rgb(0xdd6f6f),
	},
	{
		Name: "Ocean", Dark: true,
		Background: rgb(0x081620), Surface: rgb(0x0f2430),
		Text: rgb(0xdceaf2), Muted: rgb(0x7d9aa8),
		Accent: rgb(0x36b6d6), Accent2: rgb(0x58e0c0),
		Success: rgb(0x4fc98a), Warning: rgb(0xe0b25c), Danger: rgb(0xe86a7c),
	},
	{
		Name: "Plum", Dark: true,
		Background: rgb(0x150f1a), Surface: rgb(0x221830),
		Text: rgb(0xece4f5), Muted: rgb(0x9b8fae),
		Accent: rgb(0xd86fd8), Accent2: rgb(0x8c7bff),
		Success: rgb(0x72c98c), Warning: rgb(0xdfa94f), Danger: rgb(0xe8607f),
	},
	{
		Name: "Carbon", Dark: true,
		Background: rgb(0x000000), Surface: rgb(0x121212),
		Text: rgb(0xf2f2f2), Muted: rgb(0x8c8c8c),
		Accent: rgb(0xffffff), Accent2: rgb(0xb4b4b4),
		Success: rgb(0x7ac77a), Warning: rgb(0xd6b45c), Danger: rgb(0xdc6a6a),
	},
	{
		Name: "Phosphor", Dark: true,
		Background: rgb(0x050a05), Surface: rgb(0x0b140b),
		Text: rgb(0x5cff6e), Muted: rgb(0x2f8a3a),
		Accent: rgb(0x9dffa6), Accent2: rgb(0x3fd158),
		Success: rgb(0x5cff6e), Warning: rgb(0xd8ff5c), Danger: rgb(0xff5c5c),
	},
	{
		Name: "Amber", Dark: true,
		Background: rgb(0x0d0800), Surface: rgb(0x170f02),
		Text: rgb(0xffb000), Muted: rgb(0x9a6a08),
		Accent: rgb(0xffd06a), Accent2: rgb(0xff8c00),
		Success: rgb(0xffd06a), Warning: rgb(0xff8c00), Danger: rgb(0xff4a2a),
	},
	{
		Name: "Graphite", Dark: true,
		Background: rgb(0x1b1b1a), Surface: rgb(0x272725),
		Text: rgb(0xe9e6df), Muted: rgb(0x8f8c85),
		Accent: rgb(0xff6a1a), Accent2: rgb(0x7fb39c),
		Success: rgb(0x7fb39c), Warning: rgb(0xe8b23a), Danger: rgb(0xe8553a),
	},
	{
		Name: "Synthwave", Dark: true,
		Background: rgb(0x0d0221), Surface: rgb(0x1a0b3a),
		Text: rgb(0xf6e7ff), Muted: rgb(0xa58fcc),
		Accent: rgb(0xff2a6d), Accent2: rgb(0x05d9e8),
		Success: rgb(0x05d9a0), Warning: rgb(0xffc857), Danger: rgb(0xff4a4a),
	},
	{
		Name: "Paper", Dark: false,
		Background: rgb(0xf7f5f1), Surface: rgb(0xffffff),
		Text: rgb(0x22252a), Muted: rgb(0x6d7480),
		Accent: rgb(0x2f6fdb), Accent2: rgb(0x7a5af0),
		Success: rgb(0x2e9e57), Warning: rgb(0xb8791a), Danger: rgb(0xd13b3b),
	},
	{
		Name: "Linen", Dark: false,
		Background: rgb(0xf3ece2), Surface: rgb(0xfdf8f1),
		Text: rgb(0x2b2622), Muted: rgb(0x7b6f63),
		Accent: rgb(0xb5652f), Accent2: rgb(0x7d8a4a),
		Success: rgb(0x4f8a3d), Warning: rgb(0xb07d1e), Danger: rgb(0xc04a3c),
	},
	{
		Name: "Mist", Dark: false,
		Background: rgb(0xeef2f5), Surface: rgb(0xffffff),
		Text: rgb(0x1f2933), Muted: rgb(0x66737f),
		Accent: rgb(0x2b93b6), Accent2: rgb(0x5f7fa6),
		Success: rgb(0x2f9e6b), Warning: rgb(0xb2802a), Danger: rgb(0xcb4b4b),
	},
	{
		Name: "Bloom", Dark: false,
		Background: rgb(0xfaf0f3), Surface: rgb(0xfffafc),
		Text: rgb(0x2e2329), Muted: rgb(0x7d6a73),
		Accent: rgb(0xd1547d), Accent2: rgb(0x9a6ad1),
		Success: rgb(0x4f9e6b), Warning: rgb(0xbb8226), Danger: rgb(0xcf4356),
	},
	{
		Name: "Braun", Dark: false,
		Background: rgb(0xe8e5de), Surface: rgb(0xf4f2ed),
		Text: rgb(0x1e1e1c), Muted: rgb(0x7a7770),
		Accent: rgb(0xf05a1a), Accent2: rgb(0x3d6f5d),
		Success: rgb(0x3d6f5d), Warning: rgb(0xd9a21b), Danger: rgb(0xc8361b),
	},
}

// DefaultName is the theme a device nobody has chosen for shows. Light: it is also where a missing
// or unreadable settings file lands, and a white panel reads as a device warming up where a black
// one reads as one that has not come on.
const DefaultName = "Paper"

// Default is that theme.
func Default() Theme {
	t, _ := ByName(DefaultName)
	return t
}

// Names is every theme, for offering as a setting.
func Names() []string {
	out := make([]string, 0, len(All))
	for _, t := range All {
		out = append(out, t.Name)
	}
	return out
}

// ByName finds a theme. An unknown name is not one: a device asked for something it does not have
// keeps what it is showing rather than falling back silently to something else.
func ByName(name string) (Theme, bool) {
	for _, t := range All {
		if t.Name == name {
			return t, true
		}
	}
	return Theme{}, false
}
