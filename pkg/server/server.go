package server

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"tibidle-bot/pkg/bot"
)

//go:embed index.html
var indexHTML []byte

type BotController interface {
	GetAllStatuses() []bot.AccountStatus
	SupplyTicket(accountID, ticket string) error
}

type Server struct {
	port       int
	controller BotController
	startAt    time.Time
}

func NewServer(port int, controller BotController) *Server {
	return &Server{
		port:       port,
		controller: controller,
		startAt:    time.Now(),
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleDashboard)
	mux.HandleFunc("/api/status", s.handleAPIStatus)
	mux.HandleFunc("/api/ticket", s.handleAPITicket)

	addr := fmt.Sprintf(":%d", s.port)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	data := map[string]any{
		"uptime":     time.Since(s.startAt).Round(time.Second).String(),
		"goroutines": runtime.NumGoroutine(),
		"ramAllocMB": float64(m.Alloc) / 1024.0 / 1024.0,
		"ramSysMB":   float64(m.Sys) / 1024.0 / 1024.0,
		"accounts":   s.controller.GetAllStatuses(),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) handleAPITicket(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		AccountID string `json:"accountId"`
		Ticket    string `json:"ticket"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Ticket == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "Campo 'ticket' é obrigatório"})
		return
	}

	if err := s.controller.SupplyTicket(body.AccountID, body.Ticket); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"message": "Ticket entregue ao bot com sucesso! Conectando ao jogo...",
	})
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(indexHTML)
}
