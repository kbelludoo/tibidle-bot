package protocol

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed hunts.json
var huntsCatalogRaw []byte

type HuntDefinition struct {
	ID        int      `json:"id"`
	Title     string   `json:"title"`
	LevelMax  int      `json:"levelMax"`
	MaxLure   int      `json:"maxLure"`
	Monsters  []string `json:"monsters"`
	Island    string   `json:"island"`
}

type CatalogResponse struct {
	SpriteVer int              `json:"spriteVer"`
	Hunts     []HuntDefinition `json:"hunts"`
}

var Catalog CatalogResponse

func init() {
	if len(huntsCatalogRaw) > 0 {
		_ = json.Unmarshal(huntsCatalogRaw, &Catalog)
	}
}

// FindHuntByID searches for a hunt by ID
func FindHuntByID(id int) *HuntDefinition {
	for i := range Catalog.Hunts {
		if Catalog.Hunts[i].ID == id {
			return &Catalog.Hunts[i]
		}
	}
	return nil
}

// FindHuntByName searches for a hunt by name/title substring
func FindHuntByName(query string) *HuntDefinition {
	q := strings.ToLower(strings.TrimSpace(query))
	for i := range Catalog.Hunts {
		if strings.ToLower(Catalog.Hunts[i].Title) == q {
			return &Catalog.Hunts[i]
		}
	}
	for i := range Catalog.Hunts {
		if strings.Contains(strings.ToLower(Catalog.Hunts[i].Title), q) {
			return &Catalog.Hunts[i]
		}
	}
	return nil
}
