// Package settings stores one practice: address, lists, calendar default, and logo.
package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	maxNameLen = 120
	maxAddrLen = 500
	maxItemLen = 120
	maxItems   = 200
	maxLogo    = 1536 * 1024
)

// Settings is the practice record returned to the UI.
type Settings struct {
	PracticeName string   `json:"practiceName"`
	Address      string   `json:"address"`
	Services     []string `json:"services"`
	Comments     []string `json:"comments"`
	CalendarSpan string   `json:"calendarSpan"`
	HasLogo      bool     `json:"hasLogo,omitempty"`
}

// Store is the data directory of one installation.
type Store struct {
	mu  sync.Mutex
	dir string
}

// Open creates the data directory if needed.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

func (s *Store) path() string {
	return filepath.Join(s.dir, "settings.json")
}

// Get reads the practice record. A missing file yields the defaults.
func (s *Store) Get() (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getLocked()
}

func (s *Store) getLocked() (Settings, error) {
	out := defaults()
	b, err := os.ReadFile(s.path())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			out.HasLogo = s.logoPathLocked() != ""
			return out, nil
		}
		return Settings{}, err
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return Settings{}, errors.New("Die Einstellungen konnten nicht gelesen werden")
	}
	norm, err := normalize(out)
	if err != nil {
		return Settings{}, err
	}
	norm.HasLogo = s.logoPathLocked() != ""
	return norm, nil
}

func defaults() Settings {
	return Settings{
		Services:     []string{},
		Comments:     []string{},
		CalendarSpan: "month",
	}
}

// Put validates, sorts the lists, and writes them.
func (s *Store) Put(in Settings) (Settings, error) {
	norm, err := normalize(in)
	if err != nil {
		return Settings{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := writeAtomic(s.path(), norm); err != nil {
		return Settings{}, err
	}
	norm.HasLogo = s.logoPathLocked() != ""
	return norm, nil
}

func normalize(in Settings) (Settings, error) {
	name, err := cleanLine(in.PracticeName, maxNameLen, "Der Praxisname")
	if err != nil {
		return Settings{}, err
	}
	addr, err := cleanBlock(in.Address, maxAddrLen, "Die Adresse")
	if err != nil {
		return Settings{}, err
	}
	services, err := normalizeList(in.Services)
	if err != nil {
		return Settings{}, err
	}
	comments, err := normalizeList(in.Comments)
	if err != nil {
		return Settings{}, err
	}
	span := in.CalendarSpan
	switch span {
	case "month", "quarter", "halfyear", "year":
	default:
		return Settings{}, errors.New("Die Datumsansicht ist ungültig")
	}
	return Settings{
		PracticeName: name,
		Address:      addr,
		Services:     services,
		Comments:     comments,
		CalendarSpan: span,
	}, nil
}

func cleanLine(s string, max int, label string) (string, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\r\n", "")
	s = strings.ReplaceAll(s, "\n", "")
	if utf8.RuneCountInString(s) > max {
		return "", errors.New(label + " ist zu lang")
	}
	if err := rejectControls(s, false); err != nil {
		return "", err
	}
	return s, nil
}

func cleanBlock(s string, max int, label string) (string, error) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > max {
		return "", errors.New(label + " ist zu lang")
	}
	if err := rejectControls(s, true); err != nil {
		return "", err
	}
	return s, nil
}

func rejectControls(s string, allowNL bool) error {
	for _, r := range s {
		if r == '\n' && allowNL {
			continue
		}
		if r < 0x20 || r == 0x7f {
			return errors.New("Der Text enthält ungültige Zeichen")
		}
	}
	return nil
}

func normalizeList(in []string) ([]string, error) {
	if len(in) > maxItems {
		return nil, errors.New("Eine Liste darf höchstens 200 Einträge haben")
	}
	seen := map[string]struct{}{}
	out := []string{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		s = strings.ReplaceAll(s, "\r\n", "")
		s = strings.ReplaceAll(s, "\n", "")
		if s == "" {
			continue
		}
		if utf8.RuneCountInString(s) > maxItemLen {
			return nil, errors.New("Ein Eintrag darf höchstens 120 Zeichen haben")
		}
		if err := rejectControls(s, false); err != nil {
			return nil, err
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	sortGerman(out)
	return out, nil
}

func sortGerman(items []string) {
	for i := 1; i < len(items); i++ {
		j := i
		for j > 0 && foldDE(items[j]) < foldDE(items[j-1]) {
			items[j], items[j-1] = items[j-1], items[j]
			j--
		}
	}
}

func foldDE(s string) string {
	s = strings.ToLower(s)
	r := strings.NewReplacer("ä", "a", "ö", "o", "ü", "u", "ß", "ss")
	return r.Replace(s)
}

func writeAtomic(path string, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "settings-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	bak := path + ".bak"
	_ = os.Remove(bak)
	if err := os.Rename(path, bak); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Rename(bak, path)
		return err
	}
	_ = os.Remove(bak)
	ok = true
	return nil
}

// SaveLogo stores a PNG, JPEG, GIF, or WebP and drops any previous logo.
func (s *Store) SaveLogo(data []byte) error {
	if len(data) == 0 || len(data) > maxLogo {
		return errors.New("Das Logo darf höchstens 1,5 MB groß sein")
	}
	ext, err := imageExt(data)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.removeLogosLocked(); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "logo"+ext), data, 0o644)
}

// RemoveLogo deletes the practice logo.
func (s *Store) RemoveLogo() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.removeLogosLocked()
}

func (s *Store) removeLogosLocked() error {
	for _, ext := range []string{".png", ".jpg", ".gif", ".webp"} {
		err := os.Remove(filepath.Join(s.dir, "logo"+ext))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return errors.New("Das Logo wird gerade verwendet und kann nicht ersetzt werden")
		}
	}
	return nil
}

func (s *Store) logoPathLocked() string {
	for _, ext := range []string{".png", ".jpg", ".gif", ".webp"} {
		p := filepath.Join(s.dir, "logo"+ext)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// OpenLogo opens the logo file and returns a content type.
func (s *Store) OpenLogo() (io.ReadCloser, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.logoPathLocked()
	if p == "" {
		return nil, "", os.ErrNotExist
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, "", err
	}
	var head [512]byte
	n, _ := f.Read(head[:])
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		_ = f.Close()
		return nil, "", err
	}
	return f, contentType(head[:n]), nil
}

func imageExt(data []byte) (string, error) {
	switch contentType(data) {
	case "image/png":
		return ".png", nil
	case "image/jpeg":
		return ".jpg", nil
	case "image/gif":
		return ".gif", nil
	case "image/webp":
		return ".webp", nil
	default:
		return "", errors.New("Das Logo muss eine PNG-, JPEG-, GIF- oder WebP-Datei sein")
	}
}

func contentType(data []byte) string {
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	ct := http.DetectContentType(data)
	switch ct {
	case "image/png", "image/jpeg", "image/gif":
		return ct
	default:
		return ct
	}
}
