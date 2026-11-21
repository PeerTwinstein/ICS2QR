// Package ics builds a small calendar file for one patient visit.
// Times are floating local times: a practice appointment at 08:15 stays 08:15
// on the phone, without a timezone block in the QR code.
package ics

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	_ "time/tzdata"
)

const (
	maxDurationMin = 366 * 24 * 60
	prodID         = "-//Ics2QR//DE"
)

// Event is one VEVENT.
type Event struct {
	Summary  string
	Location string
	Notes    []string
	Start    time.Time
	End      time.Time
}

// Calendar is one VCALENDAR.
type Calendar struct {
	Name   string
	Events []Event
}

// AddClock moves t by whole minutes on the clock, carrying extra days.
// A duration that crosses a daylight-saving change still ends on the clock
// time the user expects, for example 22:00 plus 10 hours is 08:00.
func AddClock(t time.Time, minutes int) time.Time {
	total := t.Hour()*60 + t.Minute() + minutes
	days := 0
	if total >= 0 {
		days = total / (24 * 60)
		total = total % (24 * 60)
	} else {
		days = -((-total + 24*60 - 1) / (24 * 60))
		total = total % (24 * 60)
		if total < 0 {
			total += 24 * 60
		}
	}
	y, m, d := t.Date()
	return time.Date(y, m, d+days, total/60, total%60, 0, 0, t.Location())
}

// Resolve turns the form fields into a start and end.
// Without endDate, the end is the start plus durationMin on the clock.
// With endDate, that calendar day is used and the clock time still comes
// from the start plus the duration. The end must be after the start.
func Resolve(date, hm string, durationMin int, endDate string) (time.Time, time.Time, error) {
	return resolveIn(date, hm, durationMin, endDate, time.Local)
}

func resolveIn(date, hm string, durationMin int, endDate string, loc *time.Location) (time.Time, time.Time, error) {
	if durationMin < 1 {
		return time.Time{}, time.Time{}, errors.New("Bitte eine Dauer von mindestens einer Minute angeben")
	}
	if durationMin > maxDurationMin {
		return time.Time{}, time.Time{}, errors.New("Die Dauer darf höchstens 366 Tage betragen")
	}
	start, err := parseDateTime(date, hm, loc)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end := AddClock(start, durationMin)
	if strings.TrimSpace(endDate) == "" {
		return start, end, nil
	}
	day, err := parseDay(endDate, loc)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("Das Enddatum ist ungültig")
	}
	end = time.Date(day.Year(), day.Month(), day.Day(), end.Hour(), end.Minute(), 0, 0, loc)
	if !end.After(start) {
		return time.Time{}, time.Time{}, errors.New("Das Ende muss nach dem Beginn liegen")
	}
	return start, end, nil
}

func parseDay(date string, loc *time.Location) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(date), loc)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

func parseDateTime(date, hm string, loc *time.Location) (time.Time, error) {
	day, err := parseDay(date, loc)
	if err != nil {
		return time.Time{}, errors.New("Das Datum ist ungültig")
	}
	h, min, err := parseHM(hm)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(day.Year(), day.Month(), day.Day(), h, min, 0, 0, loc), nil
}

func parseHM(hm string) (int, int, error) {
	hm = strings.TrimSpace(hm)
	if len(hm) == 4 && hm[1] == ':' {
		hm = "0" + hm
	}
	t, err := time.Parse("15:04", hm)
	if err != nil {
		t, err = time.Parse("15:04:05", hm)
	}
	if err != nil {
		return 0, 0, errors.New("Die Uhrzeit ist ungültig")
	}
	return t.Hour(), t.Minute(), nil
}

// Build renders a minimal ICS calendar. Line breaks are CRLF, long lines are folded.
func Build(cal Calendar) (string, error) {
	if len(cal.Events) == 0 {
		return "", errors.New("Es ist kein Termin eingetragen")
	}
	stamp := time.Now().UTC().Format("20060102T150405Z")
	var lines []string
	lines = append(lines,
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:"+prodID,
		"CALSCALE:GREGORIAN",
		"METHOD:PUBLISH",
	)
	if name := strings.TrimSpace(cal.Name); name != "" {
		lines = append(lines, "X-WR-CALNAME:"+escapeText(name))
	}
	for _, ev := range cal.Events {
		if strings.TrimSpace(ev.Summary) == "" {
			return "", errors.New("Jeder Termin braucht eine Bezeichnung")
		}
		if !ev.End.After(ev.Start) {
			return "", errors.New("Das Ende muss nach dem Beginn liegen")
		}
		uid, err := newUID()
		if err != nil {
			return "", err
		}
		lines = append(lines,
			"BEGIN:VEVENT",
			"UID:"+uid,
			"DTSTAMP:"+stamp,
			"DTSTART:"+ev.Start.Format("20060102T150405"),
			"DTEND:"+ev.End.Format("20060102T150405"),
			"SUMMARY:"+escapeText(ev.Summary),
		)
		if loc := strings.TrimSpace(ev.Location); loc != "" {
			lines = append(lines, "LOCATION:"+escapeText(oneLine(loc)))
		}
		if desc := description(ev.Notes); desc != "" {
			lines = append(lines, "DESCRIPTION:"+escapeText(desc))
		}
		lines = append(lines, "END:VEVENT")
	}
	lines = append(lines, "END:VCALENDAR")

	var b strings.Builder
	for _, line := range lines {
		b.WriteString(fold(line))
		b.WriteString("\r\n")
	}
	return b.String(), nil
}

func description(notes []string) string {
	var parts []string
	for _, n := range notes {
		n = strings.TrimSpace(n)
		if n != "" {
			parts = append(parts, n)
		}
	}
	return strings.Join(parts, "\n")
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", ", ")
	s = strings.ReplaceAll(s, "\n", ", ")
	return strings.Join(strings.Fields(s), " ")
}

func escapeText(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "\r\n", `\n`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, ";", `\;`)
	s = strings.ReplaceAll(s, ",", `\,`)
	return s
}

// fold wraps a content line at 75 octets. Continuation lines start with a space.
func fold(line string) string {
	b := []byte(line)
	if len(b) <= 75 {
		return line
	}
	var buf strings.Builder
	i := 0
	limit := 75
	first := true
	for i < len(b) {
		end := i + limit
		if end > len(b) {
			end = len(b)
		} else if end < len(b) && !utf8.RuneStart(b[end]) {
			for end > i && !utf8.RuneStart(b[end]) {
				end--
			}
			if end == i {
				end = i + limit
			}
		}
		if !first {
			buf.WriteString("\r\n ")
		}
		buf.Write(b[i:end])
		i = end
		first = false
		limit = 74
	}
	return buf.String()
}

func newUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("Kennung konnte nicht erzeugt werden")
	}
	return hex.EncodeToString(b[:]) + "@ics2qr", nil
}
