package rtspd

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"

	"github.com/ygelfand/LANovo/internal/android/prop"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/vision"
	"github.com/ygelfand/LANovo/internal/lib/onvif"
)

const (
	ONVIFPort    = 8000
	snapshotPath = "/onvif/snapshot.jpg"
)

func snapshot(w http.ResponseWriter, _ *http.Request) {
	jpeg, err := vision.Get().Still()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	_, _ = w.Write(jpeg)
}

type describer struct {
	http  *http.Server
	found *onvif.Responder
}

func profiles() []onvif.Profile { return helperProfiles() }

func describe() *describer {
	dev := config.Get().Device
	serial, _ := prop.Local.Getprop("ro.serialno")
	if serial == "" {
		serial = dev.Name
	}
	svc := &onvif.Service{Device: onvif.Device{
		Manufacturer: "Lenovo",
		Model:        dev.Model,
		Firmware:     "LANovo",
		Serial:       serial,
		Hardware:     dev.Model,
		Name:         dev.Name,
		RTSPPort:     Port,
		Profiles:     profiles,
		SnapshotPath: snapshotPath,
	}}
	mux := http.NewServeMux()
	mux.Handle(onvif.DevicePath, svc)
	mux.Handle(onvif.MediaPath, svc)
	mux.HandleFunc(snapshotPath, snapshot)

	d := &describer{http: &http.Server{Handler: mux}}
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(ONVIFPort))
	if err != nil {
		slog.Warn("onvif is not answering", "port", ONVIFPort, "err", err)
		return d
	}
	go func() {
		if err := d.http.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Warn("onvif stopped serving", "port", ONVIFPort, "err", err)
		}
	}()

	d.found = &onvif.Responder{
		Hardware: "LANovo",
		UUID:     onvif.NewUUID(),
		Name:     dev.Name,
		XAddr: func(ip net.IP) string {
			return "http://" + net.JoinHostPort(
				ip.String(),
				strconv.Itoa(ONVIFPort),
			) + onvif.DevicePath
		},
	}
	if err := d.found.Listen(); err != nil {
		slog.Warn("onvif discovery is not answering", "err", err)
		d.found = nil
	}
	return d
}

func (d *describer) close() {
	if d == nil {
		return
	}
	_ = d.http.Close()
	if d.found != nil {
		d.found.Close()
	}
}
