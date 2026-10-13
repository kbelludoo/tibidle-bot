package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Hunt struct {
	ID       int      `json:"id"`
	Title    string   `json:"title"`
	MaxLure  int      `json:"maxLure"`
	Monsters []string `json:"monsters"`
	Island   string   `json:"island"`
}

type Catalog struct {
	Hunts []Hunt `json:"hunts"`
}

func main() {
	b, err := os.ReadFile("pkg/protocol/hunts.json")
	if err != nil {
		fmt.Printf("err: %v\n", err)
		return
	}
	var cat Catalog
	if err := json.Unmarshal(b, &cat); err != nil {
		fmt.Printf("unmarshal err: %v\n", err)
		return
	}
	monsters := make(map[string][]int)
	islands := make(map[string][]Hunt)
	for _, h := range cat.Hunts {
		islands[h.Island] = append(islands[h.Island], h)
		for _, m := range h.Monsters {
			monsters[m] = append(monsters[m], h.ID)
		}
	}
	fmt.Printf("Total Hunts: %d\n", len(cat.Hunts))
	fmt.Printf("Total Unique Monsters: %d\n", len(monsters))
	for isl, list := range islands {
		fmt.Printf("Island '%s': %d hunts\n", isl, len(list))
	}
}
