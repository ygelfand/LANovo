package cast

import (
	"encoding/json"
	"net/http"
	"strings"
)

const SetupPort = 8008

const IconPath = "/setup/icon.png"

func SetupHandler(eureka func() Eureka, icon func() []byte, seen func(r *http.Request, status int)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := http.StatusNotFound
		defer func() {
			if seen != nil {
				seen(r, status)
			}
		}()
		if r.URL.Path == IconPath && icon != nil {
			if png := icon(); len(png) > 0 {
				status = http.StatusOK
				w.Header().Set("Content-Type", "image/png")
				_, _ = w.Write(png)
				return
			}
		}
		if r.URL.Path != "/setup/eureka_info" || eureka == nil {
			http.NotFound(w, r)
			return
		}
		var names []string
		if p := r.URL.Query().Get("params"); p != "" {
			names = strings.Split(p, ",")
		}
		status = http.StatusOK
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(eureka().Select(names))
	})
}
