package ics

import (
	"strings"
	"testing"
	"time"
)

func TestAddClockCrossesMidnightAndYear(t *testing.T) {
	start := time.Date(2026, 12, 31, 23, 0, 0, 0, time.UTC)
	end := AddClock(start, 120)
	want := time.Date(2027, 1, 1, 1, 0, 0, 0, time.UTC)
	if !end.Equal(want) {
		t.Fatalf("got %s, want %s", end, want)
	}
}

func TestAddClockKeepsClockTimeAcrossDST(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	// 2026-03-29 is the spring-forward day. 22:00 plus 10 clock hours is 08:00,
	// not 09:00 as a plain 10-hour duration would land after the missing hour.
	start := time.Date(2026, 3, 28, 22, 0, 0, 0, loc)
	end := AddClock(start, 10*60)
	want := time.Date(2026, 3, 29, 8, 0, 0, 0, loc)
	if end.Year() != want.Year() || end.Month() != want.Month() || end.Day() != want.Day() || end.Hour() != want.Hour() || end.Minute() != want.Minute() {
		t.Fatalf("got %s, want %s", end, want)
	}
}

func TestResolveDurationAndEndDate(t *testing.T) {
	loc := time.UTC
	start, end, err := resolveIn("2026-11-21", "8:15", 15, "", loc)
	if err != nil {
		t.Fatal(err)
	}
	if got := start.Format("20060102T150405"); got != "20261121T081500" {
		t.Fatalf("start %s", got)
	}
	if got := end.Format("20060102T150405"); got != "20261121T083000" {
		t.Fatalf("end %s", got)
	}

	_, end, err = resolveIn("2026-11-21", "22:00", 180, "", loc)
	if err != nil {
		t.Fatal(err)
	}
	if got := end.Format("20060102T150405"); got != "20261122T010000" {
		t.Fatalf("overnight end %s", got)
	}

	_, end, err = resolveIn("2026-11-21", "08:00", 30, "2026-11-23", loc)
	if err != nil {
		t.Fatal(err)
	}
	if got := end.Format("20060102T150405"); got != "20261123T083000" {
		t.Fatalf("multiday end %s", got)
	}

	if _, _, err = resolveIn("2026-11-23", "08:00", 30, "2026-11-21", loc); err == nil {
		t.Fatal("expected end before start to fail")
	}
	if _, _, err = resolveIn("2026-11-21", "08:00", 0, "", loc); err == nil {
		t.Fatal("expected zero duration to fail")
	}
	if _, _, err = resolveIn("2026-02-31", "08:00", 30, "", loc); err == nil {
		t.Fatal("expected invalid date to fail")
	}
}

func TestBuildMinimalCalendar(t *testing.T) {
	start := time.Date(2026, 11, 21, 8, 15, 0, 0, time.UTC)
	end := time.Date(2026, 11, 21, 8, 30, 0, 0, time.UTC)
	out, err := Build(Calendar{
		Name: "Praxis Sonnenschein",
		Events: []Event{{
			Summary:  "Blut abnehmen",
			Location: "Hauptstraße 1\n12345 Ort",
			Notes:    []string{"Nüchtern erscheinen", "Ausweis mitbringen"},
			Start:    start,
			End:      end,
		}, {
			Summary: "Lungenfunktion",
			Start:   end,
			End:     end.Add(30 * time.Minute),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(out); i++ {
		if out[i] == '\n' && (i == 0 || out[i-1] != '\r') {
			t.Fatal("bare line feed")
		}
	}
	for _, part := range []string{
		"BEGIN:VCALENDAR\r\n",
		"PRODID:-//Ics2QR//DE\r\n",
		"X-WR-CALNAME:Praxis Sonnenschein\r\n",
		"DTSTART:20261121T081500\r\n",
		"DTEND:20261121T083000\r\n",
		"SUMMARY:Blut abnehmen\r\n",
		"LOCATION:Hauptstraße 1\\, 12345 Ort\r\n",
		`DESCRIPTION:Nüchtern erscheinen\nAusweis mitbringen` + "\r\n",
		"SUMMARY:Lungenfunktion\r\n",
		"END:VCALENDAR\r\n",
	} {
		if !strings.Contains(out, part) {
			t.Fatalf("missing %q\n%s", part, out)
		}
	}
	if strings.Count(out, "BEGIN:VEVENT") != 2 {
		t.Fatalf("events: %s", out)
	}
	if strings.Contains(out, "LOCATION") && strings.Contains(out, "Hauptstraße 1\n") {
		t.Fatal("location kept a raw newline")
	}
}

func TestEscapeAndFold(t *testing.T) {
	start := time.Date(2026, 11, 21, 8, 0, 0, 0, time.UTC)
	note := strings.Repeat("ä", 80) + ", Semikolon; und \\Pfad"
	out, err := Build(Calendar{Events: []Event{{
		Summary: "A, B; C\\D",
		Notes:   []string{note},
		Start:   start,
		End:     start.Add(time.Hour),
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `SUMMARY:A\, B\; C\\D`) {
		t.Fatalf("escape: %s", out)
	}
	if !strings.Contains(out, "\r\n ") {
		t.Fatal("expected a folded line")
	}
	flat := strings.ReplaceAll(out, "\r\n ", "")
	if !strings.Contains(flat, escapeText(note)) {
		t.Fatal("folded text lost content")
	}
	for _, line := range strings.Split(out, "\r\n") {
		if strings.HasPrefix(line, " ") {
			line = line[1:]
		}
		if len([]byte(line)) > 75 {
			t.Fatalf("line longer than 75 octets: %d %q", len([]byte(line)), line)
		}
	}
}
