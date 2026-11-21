# Ics2QR

Ics2QR ist eine kleine lokale Web-Anwendung für Praxen. An der Anmeldung werden Termine erfasst, daraus entsteht ein QR-Code mit einem Kalender im ICS-Format. Der Patient scannt den Code und übernimmt die Termine in sein Kalenderprogramm.

Die Anwendung läuft als Programm im Vordergrund oder als Dienst, unter Windows und unter Linux. Sie hört standardmäßig nur auf diesem Rechner (`127.0.0.1:8080`).

Die ursprüngliche Skizze steht in [docs/idee.md](docs/idee.md).

## Start

Go 1.26 wird vorausgesetzt.

```text
go run ./cmd/ics2qr
```

Der Browser öffnet sich mit der Termineingabe. Praxisdaten und Logo liegen im Verzeichnis `data` neben dem Programm. Dieses Verzeichnis bleibt auf dem Rechner.

Weitere Schalter:

- `-addr 127.0.0.1:8080` — Adresse, auf der der Server lauscht
- `-data PFAD` — Datenverzeichnis, sonst `data` neben dem Programm
- `-open=false` — Browser nicht öffnen
- `-service install` — Dienst einrichten; weiter möglich sind `uninstall`, `start`, `stop` und `restart`

Als Dienst startet das Programm ohne Browser.

## Bedienung

**Termine.** Datum wählen, Leistung, Uhrzeit und Dauer eintragen. Dauert ein Termin über den Tag hinaus, lässt sich ein anderes Enddatum setzen. Kommentare kommen aus der vorbereiteten Liste oder als Freitext. Mehrere Termine landen in einem QR-Code, höchstens 30. Den Code zeigt die Seite selbst, und auf Wunsch ein zweites Fenster, die Patientenanzeige. Die ICS-Datei lässt sich auch speichern.

Gespeichert werden Praxisdaten und Logo. Die einzelnen Termine bleiben in der aktuellen Sitzung. Die Patientenanzeige hält nur den gerade erzeugten QR-Code im Speicher.

**Verwaltung.** Praxisname, Adresse und Logo. Der Name erscheint in der Kopfzeile und als Kalendername, die Adresse als Ort im Termin. Das Logo (PNG, JPEG, GIF oder WebP, höchstens 1,5 MB) erscheint nur in der Oberfläche. Leistungen und Kommentare sind Listen und werden alphabetisch sortiert. Die Datumsansicht startet als Monat, Quartal, Halbjahr oder Jahr.

Die Uhrzeiten im QR-Code sind lokale Zeiten ohne Zeitzone. 08:15 bleibt 08:15, auch wenn das Telefon eine andere Zone eingestellt hat.

## Bauen und prüfen

```text
go build -o ics2qr.exe ./cmd/ics2qr
go test ./...
```

Unter Linux heißt die Datei `ics2qr`.
