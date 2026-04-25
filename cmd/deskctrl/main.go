package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"deskctrl/internal/backend"
)

//go:embed static/*
var staticFiles embed.FS

type app struct {
	system *backend.System
}

type volumePayload struct {
	Level int `json:"level"`
}

type piPromptPayload struct {
	Prompt string `json:"prompt"`
	Runner string `json:"runner"`
}

type piResultPayload struct {
	Result string `json:"result"`
}

func main() {
	addr := envOrDefault("DESKCTRL_ADDR", "127.0.0.1:47831")

	systemBackend, err := backend.NewSystem("pwsh.exe")
	if err != nil {
		log.Fatalf("init system backend: %v", err)
	}

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("load static assets: %v", err)
	}

	app := &app{
		system: systemBackend,
	}
	mux := http.NewServeMux()
	mux.Handle("/",
		http.FileServer(http.FS(staticFS)),
	)
	mux.HandleFunc("/api/screenshot", app.handleScreenshot)
	mux.HandleFunc("/api/volume", app.handleVolume)
	mux.HandleFunc("/api/pi", app.handlePi)
	mux.HandleFunc("/api/files", app.handleFiles)

	server := &http.Server{
		Addr:              addr,
		Handler:           logRequests(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("DeskCtrl listening on http://%s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("serve: %v", err)
	}
}

func (a *app) handleScreenshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	pngData, err := a.system.CaptureScreenshot(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(pngData); err != nil {
		log.Printf("write screenshot response: %v", err)
	}
}

func (a *app) handleVolume(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()

		level, err := a.system.GetVolume(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}

		writeJSON(w, http.StatusOK, volumePayload{Level: level})
	case http.MethodPost:
		var payload volumePayload
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, errors.New("invalid JSON payload"))
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()

		level, err := a.system.SetVolume(ctx, payload.Level)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}

		writeJSON(w, http.StatusOK, volumePayload{Level: level})
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (a *app) handlePi(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)

	var payload piPromptPayload
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid JSON payload"))
		return
	}

	payload.Prompt = strings.TrimSpace(payload.Prompt)
	if payload.Prompt == "" {
		writeError(w, http.StatusBadRequest, errors.New("prompt is required"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	result, err := a.system.RunPrompt(ctx, payload.Runner, payload.Prompt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, piResultPayload{Result: result})
}

func (a *app) handleFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	listing, err := a.system.ListFiles(r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, listing)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("write json response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func methodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", joinAllow(methods))
	writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
}

func joinAllow(methods []string) string {
	if len(methods) == 0 {
		return ""
	}
	result := methods[0]
	for i := 1; i < len(methods); i++ {
		result += ", " + methods[i]
	}
	return result
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		log.Printf("%s %s %s %d %s",
			r.Method,
			r.URL.Path,
			r.RemoteAddr,
			recorder.status,
			strconv.FormatInt(time.Since(start).Milliseconds(), 10)+"ms",
		)
	})
}
