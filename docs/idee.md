# Ics2QR

## Idee

Ich möchte eine kleine Web-Anwendung haben, in der man Termine eintragen kann und dann QR-Codes generieren kann. Der QR-code enthält die Daten im ICS-format, so das man es sofort mit einem Kalenderprogramm einlesen kann. Es soll ein Hilfsmittel u.a. für Praxen sein, damit die Termine direkt in den Kalendern der Patienten eingetragen werden.

### GUI

Die GUI soll sehr einfach sein. Man soll nur die Daten eingeben und dann einen QR-Code generieren können. Pro Person können es mehrere Termine sein. Sie haben immer ein Datum für den Anfang, Uhrzeit und Dauer. Geht der Termin über als einen Tag dauer, dann soll man die Dauer auch mit angeben und ein zweites Datum.

Dazu soll noch Kommentarspalte hinzukommen, zB. Dokument X mitbringen. 

### Verwaltung

Gier kann man z.B. die Adresse der Praxis eintragen. Auch für die Kommentare soll man Gruppen zun zu jeder Gruppe auch Text einfügen könne. Gruppen und Kommentare sind alphabetisch zu halten.

Der Freitext ist aber auch zu lässig. 

Auch eine Logo einer Praxis kann eingebunden werden 

### Realisierung

Den Server soll man als Programm oder Service betreiben können. Er muss unter Windows, aber auch unter Linux laufen.

Aus sicherheitsgründen wäre eine Realisierung in GO  anzustreben.
