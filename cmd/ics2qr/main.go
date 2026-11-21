// Command ics2qr serves the practice UI on this computer.
// It can run in the foreground or as a Windows service or a Linux service.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/kardianos/service"

	"ics2qr/internal/server"
	"ics2qr/internal/settings"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "Adresse, auf der der Server lauscht")
	data := flag.String("data", "", "Datenverzeichnis (Standard: data neben dem Programm)")
	open := flag.Bool("open", true, "Browser öffnen, wenn das Programm im Vordergrund läuft")
	svcCmd := flag.String("service", "", "Dienstbefehl: install, uninstall, start, stop, restart")
	flag.Parse()

	dataDir, err := resolveDataDir(*data)
	if err != nil {
		log.Fatal(err)
	}
	store, err := settings.Open(dataDir)
	if err != nil {
		log.Fatal(err)
	}

	exe, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	prg := &program{addr: *addr, store: store, openBrowser: *open}
	cfg := &service.Config{
		Name:             "Ics2QR",
		DisplayName:      "Ics2QR",
		Description:      "QR-Codes mit Kalenderterminen im ICS-Format für eine Praxis.",
		Arguments:        []string{"-addr", *addr, "-data", dataDir, "-open=false"},
		WorkingDirectory: filepath.Dir(exe),
	}
	svc, err := service.New(prg, cfg)
	if err != nil {
		log.Fatal(err)
	}
	if *svcCmd != "" {
		if err := service.Control(svc, *svcCmd); err != nil {
			log.Fatal(err)
		}
		log.Printf("Dienstbefehl %q ausgeführt", *svcCmd)
		return
	}
	if err := svc.Run(); err != nil {
		log.Fatal(err)
	}
}

func resolveDataDir(flagValue string) (string, error) {
	if strings.TrimSpace(flagValue) != "" {
		return filepath.Abs(flagValue)
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), "data"), nil
}

type program struct {
	addr        string
	store       *settings.Store
	openBrowser bool
	srv         *http.Server
}

func (p *program) Start(s service.Service) error {
	logger, _ := s.Logger(nil)
	say := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		log.Print(msg)
		if logger != nil {
			_ = logger.Info(msg)
		}
	}
	p.srv = &http.Server{
		Addr:              p.addr,
		Handler:           server.New(p.store),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       time.Minute,
	}
	go func() {
		err := p.srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			say("Serverfehler: %v", err)
		}
	}()
	url := listenURL(p.addr)
	say("Ics2QR lauscht auf %s", url)
	if p.openBrowser && service.Interactive() {
		go openURL(url)
	}
	return nil
}

func (p *program) Stop(service.Service) error {
	if p.srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return p.srv.Shutdown(ctx)
}

func listenURL(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "http://127.0.0.1" + addr
	}
	return "http://" + addr
}

func openURL(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
