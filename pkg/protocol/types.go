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

// PartyMember represents a member in the party snapshot
type PartyMember struct {
	CharacterID string `json:"characterId"`
	Name        string `json:"name"`
	Vocation    string `json:"vocation"`
	Level       int    `json:"level"`
	HP          int    `json:"hp"`
	MaxHP       int    `json:"maxHp"`
	Mana        int    `json:"mana"`
	MaxMana     int    `json:"maxMana"`
	IsLeader    bool   `json:"isLeader"`
}

// PartySnapshotPayload contains hunt title and status
type PartySnapshotPayload struct {
	Status     string        `json:"status"` // "hunting", "training", "idle", etc.
	Title      struct {
		Key    string         `json:"key"`
		Params map[string]any `json:"params"`
	} `json:"title"`
	ElapsedSec int           `json:"elapsedSec"`
	Members    []PartyMember `json:"members"`
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

// BoxSweepPayload to sweep / open loot boxes
type BoxSweepPayload struct {
	ItemID    any    `json:"itemId"`
	Count     int    `json:"count"`
	RequestID string `json:"requestId"`
}

// KeySweepPayload to sweep boss keys
type KeySweepPayload struct {
	Boss      string `json:"boss"`
	Count     int    `json:"count"`
	RequestID string `json:"requestId"`
}

// PurifyAllPayload to purify all sealed bags
type PurifyAllPayload struct {
	RequestID string `json:"requestId"`
}

// SystemMailCollectPayload to collect system mail/rewards
type SystemMailCollectPayload struct {
	MailID    string `json:"mailId"`
	RequestID string `json:"requestId"`
}

// ParcelCollectPayload to collect mailbox parcels
type ParcelCollectPayload struct {
	ParcelID  string `json:"parcelId"`
	RequestID string `json:"requestId"`
}

// EventosLoginClaimPayload to claim daily login streak reward
type EventosLoginClaimPayload struct {
	StartDate string `json:"startDate"`
	Day       int    `json:"day"`
	RequestID string `json:"requestId"`
}

// PreyBuffRollPayload to roll free daily prey buff
type PreyBuffRollPayload struct {
	Vocation  string `json:"vocation"`
	RequestID string `json:"requestId"`
}

// SetFormationPayload to set character position in party
type SetFormationPayload struct {
	Vocation string `json:"vocation"`
	Pos      int    `json:"pos"`
}

// InvasionJoinPayload to register party in daily invasion
type InvasionJoinPayload struct {
	RequestID string `json:"requestId"`
}

// DepotStoreAllPayload to stash items into depot
type DepotStoreAllPayload struct {
	RequestID string `json:"requestId"`
}

// SystemMailItem representation in system_mail_state
type SystemMailItem struct {
	ID        string `json:"id"`
	Title     string `json:"title,omitempty"`
	Collected bool   `json:"collected,omitempty"`
}

// SystemMailStatePayload received from system_mail_state
type SystemMailStatePayload struct {
	Mails        []SystemMailItem `json:"mails"`
	PendingCount int              `json:"pendingCount"`
}

// ParcelItem representation in mailbox_state
type ParcelItem struct {
	ID        string `json:"id"`
	Sender    string `json:"sender,omitempty"`
	Collected bool   `json:"collected,omitempty"`
}

// MailboxStatePayload received from mailbox_state
type MailboxStatePayload struct {
	Parcels []ParcelItem `json:"parcels"`
}

// EventosDayInfo representation of each login reward day
type EventosDayInfo struct {
	Day       int  `json:"day"`
	Resgatado bool `json:"resgatado"`
}

// EventosLoginData representation of daily streak login section
type EventosLoginData struct {
	Janela struct {
		StartDate  string           `json:"startDate"`
		TotalDays  int              `json:"totalDays"`
		IndiceHoje int              `json:"indiceHoje"`
		Dias       []EventosDayInfo `json:"dias"`
	} `json:"janela"`
	StartDate string           `json:"startDate"`
	TotalDays int              `json:"totalDays"`
	Dias      []EventosDayInfo `json:"dias"`
}

// EventosDataPayload contains daily event streak and rewards
type EventosDataPayload struct {
	Login       EventosLoginData `json:"login"`
	StartDate   string           `json:"startDate"`
	TotalDays   int              `json:"totalDays"`
	Dias        []EventosDayInfo `json:"dias"`
	Hoje        *EventosDayInfo  `json:"hoje,omitempty"`
	Resgataveis struct {
		Login     int `json:"login"`
		Diarias   int `json:"diarias"`
		Sequencia int `json:"sequencia"`
		Semanais  int `json:"semanais"`
		Total     int `json:"total"`
	} `json:"resgataveis"`
}

// InvasionTodayPayload received from invasion_today_data
type InvasionTodayPayload struct {
	Date      string `json:"date"`
	Inscricao string `json:"inscricao"` // "aberta", "inscrito", "encerrada"
	NowMs     int64  `json:"nowMs"`
}
