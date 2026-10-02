package cast

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
)

// Protocol is an application's own conversation, on namespaces beyond the media one every
// application speaks. Implementations live in their own packages and are registered on a Receiver.
type Protocol interface {
	Name() string
	Namespaces() []string

	// Receive answers one message on one of its namespaces for the running application.
	// ErrUnspoken is a message it does not handle.
	Receive(app *Application, m Message) ([]Message, error)

	// Run keeps whatever the protocol holds open until ctx ends.
	Run(ctx context.Context)
}

type Starter interface {
	Started(app App)
	Ended(app App)
}

func (r *Receiver) bracket(a App, started bool) {
	for _, p := range r.speaks(a) {
		s, ok := p.(Starter)
		switch {
		case !ok:
		case started:
			s.Started(a)
		default:
			s.Ended(a)
		}
	}
}

// ErrUnspoken is a message a protocol does not handle.
var ErrUnspoken = errors.New("cast: not a message this protocol handles")

// App is a receiver application, as a sender names it and as the panel shows it.
type App struct {
	ID   string
	Name string

	// Icon is the application's logo, empty for none.
	Icon string

	// Protocols names what it speaks beyond the media namespace.
	Protocols []string

	SupportsVideo bool
}

const Unsupported = "unsupported"

func (a App) Refused() error {
	if !slices.Contains(a.Protocols, Unsupported) {
		return nil
	}
	return fmt.Errorf("%s is not supported", a.Name)
}

// Apps are the applications this device knows by name. Names and icons are from the app config
// Google serves its devices, the icon being the default-locale promo asset.
var Apps = []App{
	{ID: "4A6A109A", Name: "AllCast", Icon: "https://lh6.ggpht.com/ZtebERyDtwAp4AM1WXg8Uxc73-cfnFmG75IZNBaP12m4HuWqpAuWkPU0VlNOZjNq25IrIKub9EGdAQ8"},
	{ID: "5E81F6DB", Name: "BBC iPlayer", Icon: "https://lh3.ggpht.com/wlITSPvvBcG2MIaA_OJRP7DNoCCNjRpm6pJ3wTCKFMAaJwFebsa6VGD8dDaFMM7lICdp5izloF-AbdzUNA"},
	{ID: "3927FA74", Name: "BubbleUPnP", Icon: "https://lh4.ggpht.com/aDqR4tFspaLkIuzly-vxvYIRGt7FBE10Y2Wf_4pp4nW3SkaT_42hwNsE9Uqbr0GPJycjJ4DvN43i-PTl"},
	{ID: "0BBDC217", Name: "CBS", Icon: "https://lh3.googleusercontent.com/m_C-xc0UEbYaqx0KoNtHoEKxSUisvBbfGXtfvBANf7v01ob5gQJmBiNOUh71v7m4wcpv0_e_zMiuLiDc"},
	{ID: "0F5096E8", Name: "Chrome Mirroring", Icon: "https://lh4.ggpht.com/x-plP9YZXhCaiDkTKQ5S29PwLmdi4feEKrMOtQle4NuoOaUgKUMH9pPWIg91da3anhSmw-G8erEIuU0d"},
	{ID: "9D8006F2", Name: "Chrome WebRTC Receiver"},     // TODO: icon
	{ID: "83384DD9", Name: "Cloud Cast (assistant)"},     // TODO: icon
	{ID: "38579375", Name: "Cloud Cast (non-assistant)"}, // TODO: icon
	{ID: "28BE5D9A", Name: "Deezer", Icon: "https://lh3.googleusercontent.com/jao2tfdDvq1QtlMixax0YcbsstGzcVDs1qhI5NIhAYEjpG95wYqt8onrqCHlAk2QCrf1si_XbZitjEo"},
	{ID: DefaultMediaReceiver, Name: "Default Media Receiver"},
	{ID: "C3A34AD9", Name: "dTV", Icon: "https://lh3.googleusercontent.com/SYsx_YtK4jagqakGh9riEI-32mM3aiuiIw5vrNVC6HJzVdbWUzkkxHd8G66zbaJOB9HmnDOmsuLR9Oprvw"},
	{ID: "08FF1091", Name: "Firefox", Icon: "https://lh6.ggpht.com/mhWlVavdsuh-rRyINqK9qHqlSsYFOPP1EbYYTk5-6XTk-7Az-LStgz1nq27k_ogzezkxV4roLvixoP6NIg"},
	{ID: "37F83649", Name: "Google Chrome for Android"}, // TODO: icon
	{ID: "5FD0CDC9", Name: "Google Photos"},             // TODO: icon
	{ID: "9381F2BD", Name: "Google Play Movies", Icon: "https://lh3.googleusercontent.com/_NaeRv3qKRVOLiE3VwsE8fx4odmFgMIrJaO3W1MVtc15RHJ056KegFMbo8fmdZZtKhTUh7XdXfib4no"},
	{ID: "58873D8A", Name: "Gospel Library"}, // TODO: icon
	{ID: "9CB11073", Name: "HBO Nordic", Icon: "https://lh3.googleusercontent.com/ihSDZwoD9grMK5Jq3_9-XiyNduj20USg-YULWlbJ1MGClxSRf_nzifCK5Q4lgCSvngayocfsIkg4t9jf"},
	{ID: "16EA8A8F", Name: "Hulu", Icon: "https://lh3.googleusercontent.com/k7faGGzd2B15LV2pIWLW3lLzCwGevqNYWxslEE1g-cMWLxuirngAy1ZHk6qVmwo_JNRMdw2zUVRLjPQ"},
	{ID: "C54DBC0F", Name: "iHeartRadio", Icon: "https://lh3.googleusercontent.com/pGmSoRCPF3PiupTEhE4CF7yqNmIe4kAE4Z-e-nv36WlFPl2AkQRBZfifgNJvlfgBuQlKVIWIIqPTGT3P"},
	{ID: "E7BE17DF", Name: "LocalCast", Icon: "https://lh3.googleusercontent.com/vxF5XS9sx4Og9U0-GHfLmO5ILZNsDBvnKDs5Ed1pko5BeRkwl85XAwrt35koZSvwUwqhrg5xBxaK4bieVQ"},
	{ID: "F3F3F51B", Name: "Music"},           // TODO: icon
	{ID: "C35B0678", Name: "Music Assistant"}, // TODO: icon
	{ID: "CA5E8412", Name: "Netflix", Icon: "https://lh3.googleusercontent.com/1LTsLJw_bvrH16FvNLggCWHQhl-lFSlFyNRJkJECYYQoPe6i7PWAK9_SEFOa0mDVD6ZJoEd9qdJDKnY"},
	{ID: "A8657F8D", Name: "NFL Now", Icon: "https://lh5.ggpht.com/xOWGwIldZza5RzJo4_cKfErSAPAjI9WofhrxuM9ZPZ4mITFppJhX-5_YQO1CEeGjKgKbV6ETTYDSUgg"},
	{ID: "6C0911F8", Name: "NOS", Icon: "https://lh3.ggpht.com/cREz3OKQtw8SNoqvB-Y8Py1Ti7DMwSm5QubShSFPvwumpgJhrByd14wWNHWbA3s-7BbYBW1wOW4nFcnD"},
	{ID: "805741C9", Name: "OneDrive", Icon: "https://lh3.googleusercontent.com/BnQQVfl3rwzW1fO0qjBQjRW9tyaOUnO6kwu5XC2YixJDXfbWopCf6mBcQKBI7rHVOu2LwkcbqfTXapsZ"},
	{ID: "36061251", Name: "Pandora", Icon: "https://lh3.googleusercontent.com/GYyq5MLoDitKbHqLQse8yGHQ6KkoVuR6satQLNR4cuIYUTBHhfRYWIWtg5A8WplV9tABNF4Xt0mPU-Yy"},
	{ID: "9AC194DC", Name: "Plex", Icon: "https://lh3.ggpht.com/F-carnehpMLNP_IxDt3-IhX9qlnPZcgnC1Ri8E9xkkkIWoeWI_GcD5ZeIQlNkKblg2jOvZLHUm-metNK"},
	{ID: "6D389446", Name: "Pocket Casts", Icon: "https://lh3.googleusercontent.com/PattgyzMJFe51uzqambxu0o4yDn7T_kc_e_IVse-rr42JHKseH3bmsyJo1_7K0oV-S7DAnpjSXY5hdkW"},
	{ID: "19DFF678", Name: "Podcast Addict", Icon: "https://lh4.ggpht.com/fUFdDbK4tLbnF3m7fZh6SecpTzLOGiGzEAnqNiuuriffXeJEw9nAcN7bQ2FEyGTlRYqBueIvvLEUOIIX"},
	{ID: "674A0243", Name: "Screen Mirroring"}, // TODO: icon
	{ID: "DD107DDB", Name: "Sendspin"},         // TODO: icon
	{ID: "DCAC4134", Name: "SoundCloud"},       // TODO: icon
	{ID: "3F70D486", Name: "SVT Play", Icon: "https://lh3.googleusercontent.com/VcPTFeK5N1xgXkqCbqj0csqkNT1qCz2MJmuAX0TAZzZsLdBNKgpdUj3Dqv4cEYxHy4eXDcvItxw8UKc"},
	{ID: "12F05308", Name: "TuneIn", Icon: "https://lh3.googleusercontent.com/HY9FJJF6gvT-JykObo1KvoNbewRoUJa2VjsE8TRgmBUmFFYGDI3FYJRGxGkj9gkMh_f3K-QSytav8G8"},
	{ID: "358E83DC", Name: "Twitch", Icon: "https://lh5.ggpht.com/Q2MqeZJTT8b1O5aV4izxhpNijdppJdb4BfcGUdYQSVuiae4RzdDoHvYDq0zEgASnDceMyNAeBRB3LDyaHA"},
	{ID: "C550AB34", Name: "Vevo", Icon: "https://lh3.googleusercontent.com/ViN1eeqqyoHzWf67OIk3n8Q-iBuYje19aRVEa1Ohlh-AtWs7AxOeAwSelgsg0scPZ3erFGA_me-dFDqP"},
	{ID: "B0E6A39F", Name: "Viaplay", Icon: "https://lh6.ggpht.com/gCBttqVDcVGZVdhHfMATAJ5Jn_EVfK2p7VUWu_ID_fmBbYk_176AdcIt_52G8qAdQ1VqPeusA11fZgFM"},
	{ID: "91AA7A8D", Name: "Video & TV Cast", Icon: "https://lh4.ggpht.com/LCVZ0aewFCyosTD1nptziidqH082v_QDvjMeVHpdjyDsd7zZMiXBcOAg0A4OnCrJqYKpmFDrZa0y9dcRkQ"},
	{ID: "2EC7CF3A", Name: "Videostream", Icon: "https://lh6.ggpht.com/ljtfN-nN_GscBAEXk0kd_yDqACjOJW27EBiONZapMeMZYmnWQgeoMgeqEk_5-Sju4Av5hZKkbex0H6qP"},
	{ID: "C193E492", Name: "WatchESPN", Icon: "https://lh4.ggpht.com/nLX7qVlXy7z2mbGU6nkFXHq6S2PTFPjXf1ZEQQ7jiuYDRMNUygTF_RLqMG6coZY5FMUtZBZgq8gXLf-Vvw"},
	{ID: "233637DE", Name: "YouTube", Icon: "https://lh3.googleusercontent.com/wAra7SVm5HwHLjGX9rVBcNeRw8YL7C8EudMZzPEfNSos21-XFSwYZGIrVxhDPBIdfoDzCoLb0_X0hz5G", Protocols: []string{"youtube"}},
	{ID: "D6EE3348", Name: "YouTube Gaming", Icon: "https://lh3.googleusercontent.com/_UlBtRouY2YYd5SD6PabZdr4rPcdvMXdZYVk52UPlDzV1Mg-3nTYX_Gp1njj0GvbFVBx0ltc60S6rII4"},
	{ID: "0354A290", Name: "YouTube Kids"}, // TODO: icon
	{ID: "2DB7CC49", Name: "YouTube Music", Icon: "https://lh3.googleusercontent.com/SS-WPDmS-pwzoqTVtESTpX0yQlsY0jp5VDZ_01-pVgK6NCBMKuQKSmprZuFYGUwPlTPfkJxJ1cqobW4", Protocols: []string{"youtube"}},
	{ID: "32EAB1DF", Name: "YouTube TV", Icon: "https://lh3.googleusercontent.com/GkrHAG8NA67MOhI51R-AgGmFJb3AKjEQEkBzkxKoJt1p26K-cVvEvbXkrxAye45aUBTkbRR2FCzaUl8U1A", Protocols: []string{"youtube"}, SupportsVideo: true},
}

var byID = sync.OnceValue(func() map[string]App {
	out := make(map[string]App, len(Apps))
	for _, a := range Apps {
		out[a.ID] = a
	}
	return out
})

// Lookup is an application by id. One not in the table runs as the default receiver would, under
// the default receiver's name.
func Lookup(id string) App {
	a, ok := byID()[id]
	if !ok {
		a = App{ID: id, Name: byID()[DefaultMediaReceiver].Name}
	}
	return a
}

// AppName is what an application is called.
func AppName(id string) string { return Lookup(id).Name }

// Register makes a protocol available to the applications that name it.
func (r *Receiver) Register(p Protocol) {
	if r.protocols == nil {
		r.protocols = map[string]Protocol{}
	}
	r.protocols[p.Name()] = p
}

// speaks is the registered protocols an application names.
func (r *Receiver) speaks(a App) []Protocol {
	var out []Protocol
	for _, name := range a.Protocols {
		if p, ok := r.protocols[name]; ok {
			out = append(out, p)
		}
	}
	return out
}

// namespaces is the media namespace and every namespace of the application's registered protocols,
// once each.
func (r *Receiver) namespaces(a App) []Namespace {
	out := []Namespace{{Name: NSMedia}}
	seen := map[string]bool{NSMedia: true}
	for _, p := range r.speaks(a) {
		for _, n := range p.Namespaces() {
			if !seen[n] {
				seen[n] = true
				out = append(out, Namespace{Name: n})
			}
		}
	}
	return out
}

// protocol is the running application's protocol that owns a namespace, if any.
func (r *Receiver) protocol(namespace string) Protocol {
	if r.app == nil {
		return nil
	}
	for _, p := range r.speaks(Lookup(r.app.AppID)) {
		for _, n := range p.Namespaces() {
			if n == namespace {
				return p
			}
		}
	}
	return nil
}
