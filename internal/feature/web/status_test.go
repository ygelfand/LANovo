package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ygelfand/libcountertop/pkg/runtime/status"
)

func TestTheRootIsTheStatusPage(t *testing.T) {
	rec := httptest.NewRecorder()
	status.Handler(page)(rec, httptest.NewRequest(http.MethodGet, "http://10.0.0.5/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "<!doctype html>") {
		t.Error("the root did not render a page")
	}
}
