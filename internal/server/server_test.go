package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ics2qr/internal/settings"
)

func TestSettingsAndQR(t *testing.T) {
	store, err := settings.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h := New(store)

	body := `{
		"practiceName": "Praxis Am See",
		"address": "Seestraße 2\n12345 Ort",
		"services": ["Lungenfunktion", "Blut abnehmen"],
		"comments": ["Nüchtern erscheinen"],
		"calendarSpan": "halfyear"
	}`
	rec := call(h, http.MethodPut, "/api/settings", strings.NewReader(body), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("put %d %s", rec.Code, rec.Body.String())
	}
	var cfg settings.Settings
	if err := json.Unmarshal(rec.Body.Bytes(), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Services[0] != "Blut abnehmen" || cfg.CalendarSpan != "halfyear" {
		t.Fatalf("%#v", cfg)
	}

	qrBody := `{
		"appointments": [{
			"summary": "Blut abnehmen",
			"date": "2026-11-21",
			"time": "08:15",
			"durationMin": 15,
			"comments": ["Nüchtern erscheinen"],
			"note": "Ausweis mitbringen"
		}]
	}`
	rec = call(h, http.MethodPost, "/api/qr", strings.NewReader(qrBody), "application/json")
	if rec.Code != http.StatusOK {
		t.Fatalf("qr %d %s", rec.Code, rec.Body.String())
	}
	var qr qrResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &qr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(qr.ICS, "SUMMARY:Blut abnehmen\r\n") || !strings.Contains(qr.ICS, "DTSTART:20261121T081500\r\n") || !strings.Contains(qr.ICS, "DTEND:20261121T083000\r\n") {
		t.Fatalf("ics:\n%s", qr.ICS)
	}
	if !strings.Contains(qr.ICS, "LOCATION:Seestraße 2\\, 12345 Ort\r\n") {
		t.Fatalf("location missing:\n%s", qr.ICS)
	}
	if !strings.Contains(qr.ICS, "X-WR-CALNAME:Praxis Am See\r\n") {
		t.Fatalf("calendar name missing:\n%s", qr.ICS)
	}
	raw, err := decodePNG(qr.PNGBase64)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 8 || string(raw[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatal("not a png")
	}

	rec = call(h, http.MethodGet, "/api/display", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("display %d %s", rec.Code, rec.Body.String())
	}
	var shown displayResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &shown); err != nil {
		t.Fatal(err)
	}
	if !shown.Active || shown.PNGBase64 != qr.PNGBase64 || shown.PracticeName != "Praxis Am See" {
		t.Fatalf("display %#v", shown)
	}
	if qr.ExpiresAt == "" || shown.ExpiresAt != qr.ExpiresAt {
		t.Fatalf("expiry qr %q display %q", qr.ExpiresAt, shown.ExpiresAt)
	}
	deadline, err := time.Parse(time.RFC3339Nano, shown.ExpiresAt)
	if err != nil {
		t.Fatal(err)
	}
	until := time.Until(deadline)
	if until < 90*time.Second || until > displayTTL+5*time.Second {
		t.Fatalf("expiry in %s", until)
	}
	etag := rec.Header().Get("ETag")
	req := httptest.NewRequest(http.MethodGet, "/api/display", nil)
	req.Header.Set("If-None-Match", etag)
	cached := httptest.NewRecorder()
	h.ServeHTTP(cached, req)
	if cached.Code != http.StatusNotModified {
		t.Fatalf("etag %d", cached.Code)
	}
	rec = call(h, http.MethodDelete, "/api/display", nil, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("clear display %d", rec.Code)
	}
	rec = call(h, http.MethodGet, "/api/display", nil, "")
	shown = displayResponse{}
	if err := json.Unmarshal(rec.Body.Bytes(), &shown); err != nil {
		t.Fatal(err)
	}
	if shown.Active || shown.PNGBase64 != "" {
		t.Fatalf("display still active %#v", shown)
	}

	rec = call(h, http.MethodGet, "/", nil, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "QR-Code erzeugen") {
		t.Fatalf("index %d", rec.Code)
	}
	rec = call(h, http.MethodGet, "/anzeige.html", nil, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Bitte mit der Kamera scannen") {
		t.Fatalf("anzeige %d", rec.Code)
	}
}

func TestLogoUpload(t *testing.T) {
	store, err := settings.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h := New(store)
	var img bytes.Buffer
	if err := png.Encode(&img, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("logo", "logo.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(img.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	rec := call(h, http.MethodPost, "/api/logo", &body, w.FormDataContentType())
	if rec.Code != http.StatusOK {
		t.Fatalf("upload %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodGet, "/logo", nil, "")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("get logo %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
}

func TestBoardExpiresAfterTwoMinutes(t *testing.T) {
	start := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)
	clock := start
	b := &board{now: func() time.Time { return clock }}

	expires := b.show("png", "Praxis")
	if !expires.Equal(start.Add(displayTTL)) {
		t.Fatalf("expires %s", expires)
	}
	_, png, name, got := b.get()
	if png != "png" || name != "Praxis" || !got.Equal(expires) {
		t.Fatalf("visible png %q name %q expires %s", png, name, got)
	}

	clock = start.Add(displayTTL - time.Second)
	if _, png, _, _ = b.get(); png != "png" {
		t.Fatal("expired one second early")
	}

	clock = start.Add(displayTTL)
	rev, png, name, got := b.get()
	if png != "" || name != "" || !got.IsZero() {
		t.Fatalf("still visible png %q name %q", png, name)
	}
	if rev2, _, _, _ := b.get(); rev2 != rev {
		t.Fatal("second read changed the revision")
	}

	clock = start.Add(displayTTL)
	b.show("erneuert", "Praxis")
	clock = start.Add(2*displayTTL - time.Second)
	if _, png, _, _ = b.get(); png != "erneuert" {
		t.Fatal("a new code reused the old deadline")
	}
	b.clear()
	clock = clock.Add(displayTTL)
	if _, png, _, _ = b.get(); png != "" {
		t.Fatal("cleared code came back")
	}
}

func TestQRRejectsEmpty(t *testing.T) {
	store, err := settings.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h := New(store)
	rec := call(h, http.MethodPost, "/api/qr", strings.NewReader(`{"appointments":[]}`), "application/json")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d", rec.Code)
	}
}

func call(h http.Handler, method, path string, body io.Reader, ct string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodePNG(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
