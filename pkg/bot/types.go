package bot

import (
	"sync"
)

type AccountConfig struct {
	ID            string `json:"id"`
	Email         string `json:"email,omitempty"`
	Password      string `json:"password,omitempty"`
	SessionCookie string `json:"sessionCookie,omitempty"`
	SessionProof  string `json:"sessionProof,omitempty"`
	Ticket        string `json:"ticket,omitempty"`
	HuntID        int    `json:"huntId,omitempty"`
	HuntName      string `json:"huntName,omitempty"`
	Lure          int    `json:"lure,omitempty"`
	AutoSell      bool   `json:"autoSell"`
	AutoBlessings bool   `json:"autoBlessings"`
	AutoDaily     bool   `json:"autoDaily"`
}

type AccountStatus struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Vocation    string   `json:"vocation"`
	Level       int      `json:"level"`
	HP          int      `json:"hp"`
	MaxHP       int      `json:"maxHp"`
	Mana        int      `json:"mana"`
	MaxMana     int      `json:"maxMana"`
	Gold        int      `json:"gold"`
	XP          string   `json:"xp"`
	Status      string   `json:"status"` // "hunting", "training", "idle", "connected", "offline"
	CurrentHunt string   `json:"currentHunt"`
	Kills       int      `json:"kills"`
	ExpPerHour  float64  `json:"expPerHour"`
	GoldPerHour float64  `json:"goldPerHour"`
	DurationSec int      `json:"durationSec"`
	UpdatedAt   int64    `json:"updatedAt"`
	RecentLogs  []string `json:"recentLogs"`
}

type SafeStatus struct {
	mu     sync.RWMutex
	status AccountStatus
}

func NewSafeStatus(id string) *SafeStatus {
	return &SafeStatus{
		status: AccountStatus{
			ID:     id,
			Status: "Iniciando…",
		},
	}
}

func (s *SafeStatus) Get() AccountStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := s.status
	logs := make([]string, len(s.status.RecentLogs))
	copy(logs, s.status.RecentLogs)
	cp.RecentLogs = logs
	return cp
}

func (s *SafeStatus) Update(fn func(st *AccountStatus)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.status)
}

func (s *SafeStatus) AddLog(entry string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.RecentLogs = append(s.status.RecentLogs, entry)
	if len(s.status.RecentLogs) > 15 {
		s.status.RecentLogs = s.status.RecentLogs[len(s.status.RecentLogs)-15:]
	}
}
