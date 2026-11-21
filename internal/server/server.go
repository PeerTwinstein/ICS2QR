// Package server is the local HTTP API and the embedded practice UI.
package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/skip2/go-qrcode"

	"ics2qr/internal/ics"
	"ics2qr/internal/settings"
	"ics2qr/web"
)

const (
	maxBody         = 1 << 20
	maxAppointments = 30
	maxSummary      = 200
	maxNote         = 500
	maxComments     = 20
	displayTTL      = 2 * time.Minute
)

// New builds the application handler. Appointments are not stored.
// The patient display keeps only the current QR code in memory and drops it after two minutes.
func New(store *settings.Store) http.Handler {
	screen := &board{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/settings", func(w http.ResponseWriter, r *http.Request) {
		got, err := store.Get()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, got)
	})
	mux.HandleFunc("PUT /api/settings", func(w http.ResponseWriter, r *http.Request) {
		var in settings.Settings
		if err := readJSON(w, r, &in); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		got, err := store.Put(in)
		if err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, got)
	})
	mux.HandleFunc("POST /api/logo", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, settingsLogoLimit())
		if err := r.ParseMultipartForm(settingsLogoLimit()); err != nil {
			writeErr(w, http.StatusBadRequest, errors.New("Das Logo konnte nicht gelesen werden"))
			return
		}
		f, _, err := r.FormFile("logo")
		if err != nil {
			writeErr(w, http.StatusBadRequest, errors.New("Bitte eine Bilddatei wählen"))
			return
		}
		defer f.Close()
		data, err := io.ReadAll(io.LimitReader(f, settingsLogoLimit()+1))
		if err != nil {
			writeErr(w, http.StatusBadRequest, errors.New("Das Logo konnte nicht gelesen werden"))
			return
		}
		if err := store.SaveLogo(data); err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"hasLogo": true})
	})
	mux.HandleFunc("DELETE /api/logo", func(w http.ResponseWriter, r *http.Request) {
		if err := store.RemoveLogo(); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"hasLogo": false})
	})
	mux.HandleFunc("GET /logo", func(w http.ResponseWriter, r *http.Request) {
		f, ct, err := store.OpenLogo()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = io.Copy(w, f)
	})
	mux.HandleFunc("POST /api/qr", func(w http.ResponseWriter, r *http.Request) {
		var in qrRequest
		if err := readJSON(w, r, &in); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		cfg, err := store.Get()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		text, err := buildICS(cfg, in.Appointments)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		png, err := encodeQR(text)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		encoded := base64.StdEncoding.EncodeToString(png)
		expires := screen.show(encoded, cfg.PracticeName)
		writeJSON(w, http.StatusOK, qrResponse{
			ICS:       text,
			PNGBase64: encoded,
			ExpiresAt: formatExpiry(expires),
		})
	})
	mux.HandleFunc("GET /api/display", func(w http.ResponseWriter, r *http.Request) {
		rev, png, name, expires := screen.get()
		tag := fmt.Sprintf(`"%d"`, rev)
		w.Header().Set("ETag", tag)
		w.Header().Set("Cache-Control", "no-store")
		if r.Header.Get("If-None-Match") == tag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		resp := displayResponse{
			Active:       png != "",
			PNGBase64:    png,
			PracticeName: name,
		}
		if !expires.IsZero() {
			resp.ExpiresAt = formatExpiry(expires)
		}
		writeJSON(w, http.StatusOK, resp)
	})
	mux.HandleFunc("DELETE /api/display", func(w http.ResponseWriter, r *http.Request) {
		screen.clear()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.Handle("GET /", staticHandler())
	return secure(mux)
}

func settingsLogoLimit() int64 {
	return 1536*1024 + 4096
}

type qrRequest struct {
	Appointments []apptIn `json:"appointments"`
}

type apptIn struct {
	Summary     string   `json:"summary"`
	Date        string   `json:"date"`
	Time        string   `json:"time"`
	DurationMin int      `json:"durationMin"`
	EndDate     string   `json:"endDate"`
	Comments    []string `json:"comments"`
	Note        string   `json:"note"`
}

type qrResponse struct {
	ICS       string `json:"ics"`
	PNGBase64 string `json:"pngBase64"`
	ExpiresAt string `json:"expiresAt"`
}

type displayResponse struct {
	Active       bool   `json:"active"`
	PNGBase64    string `json:"pngBase64,omitempty"`
	PracticeName string `json:"practiceName,omitempty"`
	ExpiresAt    string `json:"expiresAt,omitempty"`
}

// board is the QR code currently shown to the patient.
// It is not written to disk and disappears after displayTTL.
type board struct {
	mu       sync.Mutex
	revision uint64
	png      string
	name     string
	expires  time.Time
	now      func() time.Time
}

func (b *board) clock() time.Time {
	if b.now != nil {
		return b.now()
	}
	return time.Now()
}

func (b *board) show(png, name string) time.Time {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.revision++
	b.png = png
	b.name = name
	b.expires = b.clock().Add(displayTTL)
	return b.expires
}

func (b *board) clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.png == "" {
		return
	}
	b.revision++
	b.png = ""
	b.name = ""
	b.expires = time.Time{}
}

func (b *board) get() (uint64, string, string, time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.expireLocked()
	return b.revision, b.png, b.name, b.expires
}

func formatExpiry(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z07:00")
}

func (b *board) expireLocked() {
	if b.png == "" || b.expires.IsZero() || b.clock().Before(b.expires) {
		return
	}
	b.revision++
	b.png = ""
	b.name = ""
	b.expires = time.Time{}
}

func buildICS(cfg settings.Settings, apps []apptIn) (string, error) {
	if len(apps) == 0 {
		return "", errors.New("Es ist kein Termin eingetragen")
	}
	if len(apps) > maxAppointments {
		return "", errors.New("Es sind höchstens 30 Termine in einem QR-Code möglich")
	}
	events := make([]ics.Event, 0, len(apps))
	for _, a := range apps {
		summary := strings.TrimSpace(a.Summary)
		if summary == "" {
			return "", errors.New("Jeder Termin braucht eine Bezeichnung")
		}
		if utf8.RuneCountInString(summary) > maxSummary {
			return "", errors.New("Die Bezeichnung ist zu lang")
		}
		note := strings.TrimSpace(a.Note)
		if utf8.RuneCountInString(note) > maxNote {
			return "", errors.New("Der Freitext ist zu lang")
		}
		if len(a.Comments) > maxComments {
			return "", errors.New("Zu viele Kommentare an einem Termin")
		}
		var notes []string
		for _, c := range a.Comments {
			c = strings.TrimSpace(c)
			if c == "" {
				continue
			}
			if utf8.RuneCountInString(c) > maxSummary {
				return "", errors.New("Ein Kommentar ist zu lang")
			}
			notes = append(notes, c)
		}
		if note != "" {
			notes = append(notes, note)
		}
		start, end, err := ics.Resolve(a.Date, a.Time, a.DurationMin, a.EndDate)
		if err != nil {
			return "", err
		}
		events = append(events, ics.Event{
			Summary:  summary,
			Location: cfg.Address,
			Notes:    notes,
			Start:    start,
			End:      end,
		})
	}
	return ics.Build(ics.Calendar{Name: cfg.PracticeName, Events: events})
}

func encodeQR(text string) ([]byte, error) {
	for _, level := range []qrcode.RecoveryLevel{qrcode.Medium, qrcode.Low} {
		png, err := qrcode.Encode(text, level, 560)
		if err == nil {
			return png, nil
		}
	}
	return nil, errors.New("Der Inhalt passt nicht in einen QR-Code. Kürzen Sie die Texte oder teilen Sie die Termine auf")
}

func readJSON(w http.ResponseWriter, r *http.Request, dest any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		return errors.New("Die Eingabe konnte nicht gelesen werden")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func statusFor(err error) int {
	msg := err.Error()
	if strings.Contains(msg, "zu lang") || strings.Contains(msg, "ungültig") || strings.Contains(msg, "höchstens") || strings.Contains(msg, "Logo") || strings.Contains(msg, "Bitte") || strings.Contains(msg, "muss") {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func staticHandler() http.Handler {
	fs := http.FileServer(http.FS(web.Files))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		fs.ServeHTTP(w, r)
	})
}

func secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; base-uri 'none'; form-action 'self'")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}
