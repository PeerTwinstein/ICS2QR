package settings

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestPutSortsGermanAndDropsDuplicates(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := st.Put(Settings{
		PracticeName: "  Praxis Am See  ",
		Address:      "Seestraße 2\n12345 Ort",
		Services:     []string{"Übungen", "Auswertung", "Ärztliche Beratung", "auswertung"},
		Comments:     []string{"Nüchtern erscheinen", "Dokument mitbringen"},
		CalendarSpan: "quarter",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Ärztliche Beratung", "Auswertung", "Übungen"}
	if stringsJoin(got.Services) != stringsJoin(want) {
		t.Fatalf("services %#v", got.Services)
	}
	if got.PracticeName != "Praxis Am See" || got.CalendarSpan != "quarter" || got.HasLogo {
		t.Fatalf("%#v", got)
	}
	again, err := st.Get()
	if err != nil {
		t.Fatal(err)
	}
	if stringsJoin(again.Services) != stringsJoin(want) {
		t.Fatalf("reread %#v", again.Services)
	}
	if _, err := os.Stat(filepath.Join(dir, "settings.json")); err != nil {
		t.Fatal(err)
	}
}

func TestRejectsBadSpanAndHugeItem(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Put(Settings{CalendarSpan: "week", Services: []string{}, Comments: []string{}}); err == nil {
		t.Fatal("expected bad span")
	}
	long := make([]rune, 121)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := st.Put(Settings{CalendarSpan: "month", Services: []string{string(long)}, Comments: []string{}}); err == nil {
		t.Fatal("expected long item to fail")
	}
}

func TestLogoRoundTrip(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveLogo(buf.Bytes()); err != nil {
		t.Fatal(err)
	}
	f, ct, err := st.OpenLogo()
	if err != nil {
		t.Fatal(err)
	}
	if ct != "image/png" {
		f.Close()
		t.Fatalf("content type %s", ct)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := st.Get()
	if err != nil || !got.HasLogo {
		t.Fatalf("hasLogo %v %v", got.HasLogo, err)
	}
	if err := st.SaveLogo([]byte("kein bild")); err == nil {
		t.Fatal("expected reject")
	}
	if err := st.RemoveLogo(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.OpenLogo(); !os.IsNotExist(err) {
		t.Fatalf("still present: %v", err)
	}
}

func stringsJoin(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += "|"
		}
		out += v
	}
	return out
}
