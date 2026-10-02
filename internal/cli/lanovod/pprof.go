package lanovod

import (
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"time"
)

// The profiler, off unless asked for.
//
// Answering "what is the device doing with the CPU" by elimination costs a lot of turns and gets
// it wrong; a profile names the function. Bound to the loopback, so reaching it means `adb forward
// tcp:6060 tcp:6060` and then the usual `go tool pprof`.
const pprofAddr = "127.0.0.1:6060"

func startPprof() {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	l, err := net.Listen("tcp", pprofAddr)
	if err != nil {
		slog.Error("could not open the profiler", "addr", pprofAddr, "err", err)
		return
	}
	slog.Warn("the profiler is open", "addr", pprofAddr)

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := srv.Serve(l); err != nil && err != http.ErrServerClosed {
			slog.Error("the profiler stopped", "err", err)
		}
	}()
}
