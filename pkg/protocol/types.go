package protocol

import "encoding/json"

// InboundMessage represents any message received from the Tibidle WebSocket server.
type InboundMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// OutboundMessage represents any message sent to the Tibidle WebSocket server.
type OutboundMessage struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// AuthPayload sent immediately upon WebSocket connection.
type AuthPayload struct {
	Ticket           string `json:"ticket"`
	TradeFlowVersion int    `json:"tradeFlowVersion"`
}

// StartHuntPayload to initiate a hunt.
type StartHuntPayload struct {
	HuntID   int    `json:"huntId"`
	Lure     int    `json:"lure"`
	BossID   string `json:"bossId,omitempty"`
	AutoBoss bool   `json:"autoBoss,omitempty"`
}

// StopPayload to stop current hunt.
type StopPayload struct {
	Intent string `json:"intent,omitempty"`
}

// SellItem item to be sold.
type SellItem struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
	IID   string `json:"iid,omitempty"`
}

// SellLootPayload to sell inventory loot.
type SellLootPayload struct {
	Items     []SellItem `json:"items"`
	RequestID string     `json:"requestId"`
}

// BuyBlessingsPayload to buy all protections.
type BuyBlessingsPayload struct {
	RequestID string `json:"requestId"`
}

// EventosClaimPayload to claim daily event tasks.
type EventosClaimPayload struct {
	Slot      int    `json:"slot"`
	RequestID string `json:"requestId"`
}

// StartTrainPayload to start character training.
type StartTrainPayload struct {
	PerMember []MemberTrainConfig `json:"perMember"`
}

type MemberTrainConfig struct {
	Vocation string `json:"vocation"`
	Mode     string `json:"mode"`
	Item     string `json:"item,omitempty"`
	IID      string `json:"iid,omitempty"`
}

// CharacterStats received inside Frame
type CharacterStats struct {
	Name     string `json:"name"`
	Vocation string `json:"vocation"`
	Sex      string `json:"sex"`
	Level    int    `json:"level"`
	HP       int    `json:"hp"`
	MaxHP    int    `json:"maxHp"`
	Mana     int    `json:"mana"`
	MaxMana  int    `json:"maxMana"`
	XP       string `json:"xp"`
	Gold     int    `json:"gold"`
}

// AnalyzerStats received inside Frame
type AnalyzerStats struct {
	ElapsedMs   int            `json:"elapsedMs"`
	XP          string         `json:"xp"`
	XpPerHour   int            `json:"xpPerHour"`
	Kills       map[string]int `json:"kills"`
	KillsTotal  int            `json:"killsTotal"`
	DamageDealt int            `json:"damageDealt"`
	DamageTaken int            `json:"damageTaken"`
	HealingDone int            `json:"healingDone"`
}

// CapacityStats represents character capacity
type CapacityStats struct {
	Used int `json:"used"`
	Max  int `json:"max"`
}

// InventoryItem represents an item in character inventory
type InventoryItem struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
	IID   string `json:"iid"`
}

// FramePayload contains the live frame data
type FramePayload struct {
	State struct {
		Character CharacterStats  `json:"character"`
		Cap       CapacityStats   `json:"cap"`
		Inventory []InventoryItem `json:"inventory"`
	} `json:"state"`
	Analyzer AnalyzerStats `json:"analyzer"`
}

// PartySnapshotPayload contains hunt title and status
type PartySnapshotPayload struct {
	Status string `json:"status"` // "hunting", "training", "idle", etc.
	Title  struct {
		Key    string         `json:"key"`
		Params map[string]any `json:"params"`
	} `json:"title"`
	ElapsedSec int `json:"elapsedSec"`
}

// ResumePayload received when continuing an ongoing hunt
type ResumePayload struct {
	HuntID int `json:"huntId"`
	State  struct {
		Character CharacterStats `json:"character"`
	} `json:"state"`
}

// EventosStatePayload holds server event progress
type EventosStatePayload struct {
	RequestID string `json:"requestId"`
	Action    string `json:"action"`
	OK        bool   `json:"ok"`
	State     struct {
		ServerDay string `json:"serverDay"`
	} `json:"state"`
}
