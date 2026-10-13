package bot

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"tibidle-bot/pkg/protocol"
)

// BestiaryFileStructure represents the persisted bestiary progress on disk.
type BestiaryDiskData struct {
	AccountID      string         `json:"accountId"`
	UpdatedAt      int64          `json:"updatedAt"`
	Kills          map[string]int `json:"kills"`
	KnownMinLevels map[int]int    `json:"knownMinLevels,omitempty"`
}

var (
	bestiaryMu sync.Mutex
)

// AllBestiaryHunts contains all 113 hunts in Tibidle, sorted by progression level brackets.
var AllBestiaryHunts = []HuntTier{
	// Tier 1: Level 1 - 20 (Early Game / Rook & Starter Caves)
	{MinLevel: 1, HuntID: 26, HuntName: "Daramian Minotaur Pyramid", Lure: 2},
	{MinLevel: 1, HuntID: 170, HuntName: "Wolves Den", Lure: 2},
	{MinLevel: 1, HuntID: 176, HuntName: "Wasps Dungeons", Lure: 2},
	{MinLevel: 1, HuntID: 125, HuntName: "Goblins Femor Hills", Lure: 2},
	{MinLevel: 1, HuntID: 126, HuntName: "Rotworms Darashia", Lure: 2},
	{MinLevel: 5, HuntID: 178, HuntName: "Salamander Cave", Lure: 2},
	{MinLevel: 10, HuntID: 131, HuntName: "Dwarven Mines", Lure: 2},
	{MinLevel: 10, HuntID: 28, HuntName: "Tarpit Tomb First Floor", Lure: 1},
	{MinLevel: 10, HuntID: 175, HuntName: "Carlin Raids", Lure: 2},
	{MinLevel: 15, HuntID: 128, HuntName: "Larva Ankrahmuns", Lure: 2},
	{MinLevel: 15, HuntID: 124, HuntName: "Orcs Edron Cave", Lure: 2},

	// Tier 2: Level 20 - 40 (Low-Mid Tier Hunts)
	{MinLevel: 20, HuntID: 139, HuntName: "Crocodiles Port Hope", Lure: 3},
	{MinLevel: 20, HuntID: 180, HuntName: "Amazon Camp", Lure: 1},
	{MinLevel: 20, HuntID: 119, HuntName: "Dwarf Bridge", Lure: 2},
	{MinLevel: 20, HuntID: 194, HuntName: "Desert Quest", Lure: 1},
	{MinLevel: 25, HuntID: 32, HuntName: "Elfs Shadowthorn", Lure: 3},
	{MinLevel: 25, HuntID: 132, HuntName: "Elfs Yalahar", Lure: 2},
	{MinLevel: 25, HuntID: 134, HuntName: "Dworcs Port Hope", Lure: 3},
	{MinLevel: 25, HuntID: 135, HuntName: "Chakoyas Svargrond", Lure: 1},
	{MinLevel: 25, HuntID: 138, HuntName: "Terramites Ankrahmun", Lure: 1},
	{MinLevel: 25, HuntID: 30, HuntName: "Coryms Lair", Lure: 1},
	{MinLevel: 30, HuntID: 141, HuntName: "Cyclops Thais", Lure: 1},
	{MinLevel: 30, HuntID: 38, HuntName: "Cyclops Mistrock", Lure: 1},
	{MinLevel: 35, HuntID: 142, HuntName: "Barbarian Camp", Lure: 3},
	{MinLevel: 35, HuntID: 34, HuntName: "Orc Fortress", Lure: 1},

	// Tier 3: Level 40 - 60 (Mid Tier Dungeons)
	{MinLevel: 40, HuntID: 102, HuntName: "Mutated Humans", Lure: 1},
	{MinLevel: 40, HuntID: 109, HuntName: "Gargoyles Meriana", Lure: 2},
	{MinLevel: 40, HuntID: 110, HuntName: "Tortoise Meriana", Lure: 2},
	{MinLevel: 40, HuntID: 144, HuntName: "Zombies Yalahar Cemetery Quarter", Lure: 1},
	{MinLevel: 45, HuntID: 140, HuntName: "Scarabs Cave", Lure: 2},
	{MinLevel: 45, HuntID: 101, HuntName: "Pirates Yalahar", Lure: 1},
	{MinLevel: 45, HuntID: 103, HuntName: "Nargor Pirate island", Lure: 3},
	{MinLevel: 45, HuntID: 143, HuntName: "Water Elementals Cavern", Lure: 1},
	{MinLevel: 45, HuntID: 166, HuntName: "Cults Carlin", Lure: 2},
	{MinLevel: 50, HuntID: 43, HuntName: "Cults Goroma", Lure: 2},
	{MinLevel: 50, HuntID: 45, HuntName: "Foreign Quarter", Lure: 1},
	{MinLevel: 50, HuntID: 146, HuntName: "Djinns Marid Territory", Lure: 2},
	{MinLevel: 50, HuntID: 195, HuntName: "Mad Mage Room", Lure: 1},
	{MinLevel: 50, HuntID: 217, HuntName: "Black Knight Quest", Lure: 1},
	{MinLevel: 55, HuntID: 162, HuntName: "Werebadgers Nest", Lure: 4},
	{MinLevel: 55, HuntID: 163, HuntName: "Feyrist Surface", Lure: 4},

	// Tier 4: Level 60 - 80 (Advanced Hunting Grounds)
	{MinLevel: 60, HuntID: 46, HuntName: "Dragon Lair", Lure: 4},
	{MinLevel: 60, HuntID: 149, HuntName: "Ice Witch Tower", Lure: 4},
	{MinLevel: 60, HuntID: 151, HuntName: "Bog Raiders Yalahar", Lure: 4},
	{MinLevel: 60, HuntID: 155, HuntName: "Stonerefiners Cavern", Lure: 3},
	{MinLevel: 60, HuntID: 186, HuntName: "Zao Entrance", Lure: 4},
	{MinLevel: 60, HuntID: 187, HuntName: "The Banshee Quest", Lure: 1},
	{MinLevel: 60, HuntID: 189, HuntName: "Gladiators Arena Yalahar", Lure: 1},
	{MinLevel: 65, HuntID: 165, HuntName: "Zao Caverns", Lure: 4},
	{MinLevel: 65, HuntID: 157, HuntName: "Killer Caiman Caverns", Lure: 2},
	{MinLevel: 65, HuntID: 182, HuntName: "Return of the beholder", Lure: 4},
	{MinLevel: 65, HuntID: 190, HuntName: "Quara's entrance Yalahar", Lure: 3},
	{MinLevel: 70, HuntID: 161, HuntName: "Yalahar Worker Golem", Lure: 4},
	{MinLevel: 75, HuntID: 105, HuntName: "Giant Spiders POH", Lure: 4},
	{MinLevel: 75, HuntID: 106, HuntName: "Hero Fortress Entrance", Lure: 4},
	{MinLevel: 75, HuntID: 183, HuntName: "Thunderscar Peak", Lure: 4},
	{MinLevel: 75, HuntID: 184, HuntName: "Petrified Hollow", Lure: 4},

	// Tier 5: Level 80 - 100 (High Level Zones)
	{MinLevel: 80, HuntID: 50, HuntName: "Vampire hell", Lure: 1},
	{MinLevel: 80, HuntID: 99, HuntName: "Mintwallin", Lure: 3},
	{MinLevel: 80, HuntID: 156, HuntName: "Souleater's Canyon", Lure: 4},
	{MinLevel: 80, HuntID: 191, HuntName: "Quara's Domain Yalahar", Lure: 4},
	{MinLevel: 80, HuntID: 193, HuntName: "Fiehonja Stairway", Lure: 2},
	{MinLevel: 80, HuntID: 123, HuntName: "The Hive Entrance", Lure: 4},
	{MinLevel: 85, HuntID: 53, HuntName: "Frozen Hell", Lure: 4},
	{MinLevel: 85, HuntID: 111, HuntName: "Hellspawns yalahar", Lure: 4},
	{MinLevel: 85, HuntID: 198, HuntName: "Hive Tower", Lure: 4},
	{MinLevel: 85, HuntID: 199, HuntName: "Hive Queen Chamber", Lure: 4},
	{MinLevel: 90, HuntID: 49, HuntName: "Pits of Inferno Entrance", Lure: 4},
	{MinLevel: 90, HuntID: 62, HuntName: "Lizard City", Lure: 4},
	{MinLevel: 95, HuntID: 107, HuntName: "Lizard Chosen's Dojo", Lure: 4},

	// Tier 6: Level 100 - 130 (Endgame Pre-Warzone)
	{MinLevel: 100, HuntID: 188, HuntName: "The Behemoth Quest", Lure: 1},
	{MinLevel: 100, HuntID: 196, HuntName: "Deepling Library", Lure: 4},
	{MinLevel: 100, HuntID: 197, HuntName: "Bottom of the Ocean", Lure: 4},
	{MinLevel: 100, HuntID: 220, HuntName: "Inquisition Warlocks", Lure: 1},
	{MinLevel: 100, HuntID: 221, HuntName: "Inquisition Entrance", Lure: 1},
	{MinLevel: 100, HuntID: 226, HuntName: "Fire Portal", Lure: 1},
	{MinLevel: 100, HuntID: 228, HuntName: "Holy Portal", Lure: 4},
	{MinLevel: 100, HuntID: 229, HuntName: "Otherworld Kazordoon", Lure: 2},
	{MinLevel: 100, HuntID: 231, HuntName: "Otherworld Ankrahmun", Lure: 1},
	{MinLevel: 100, HuntID: 232, HuntName: "Otherworld Svargrond", Lure: 1},
	{MinLevel: 100, HuntID: 234, HuntName: "Lower Spikes", Lure: 4},
	{MinLevel: 100, HuntID: 235, HuntName: "Upper Spikes", Lure: 4},
	{MinLevel: 100, HuntID: 236, HuntName: "Middle Spikes", Lure: 4},
	{MinLevel: 110, HuntID: 192, HuntName: "Draken Walls", Lure: 4},

	// Tier 7: Level 130+ (Warzones, PoI Thrones, Inquisition Seals, Portals)
	{MinLevel: 130, HuntID: 148, HuntName: "Warzone - Lava Lurkers", Lure: 4},
	{MinLevel: 130, HuntID: 214, HuntName: "Warzone I - Fungus", Lure: 1},
	{MinLevel: 130, HuntID: 238, HuntName: "Warzone I - Montanhas Rochosas", Lure: 1},
	{MinLevel: 130, HuntID: 215, HuntName: "Warzone II - Magma Dungeon", Lure: 1},
	{MinLevel: 130, HuntID: 239, HuntName: "Warzone II - Ninho de Parasitas", Lure: 4},
	{MinLevel: 130, HuntID: 65, HuntName: "Warzone III - Crystal Caves", Lure: 2},
	{MinLevel: 130, HuntID: 237, HuntName: "Crystal Keeper", Lure: 4},
	{MinLevel: 130, HuntID: 240, HuntName: "Mushroom Gardens", Lure: 4},
	{MinLevel: 130, HuntID: 241, HuntName: "Lava Hot Spot", Lure: 4},
	{MinLevel: 130, HuntID: 200, HuntName: "Throne of Verminor", Lure: 4},
	{MinLevel: 130, HuntID: 201, HuntName: "Throne of Infernatil", Lure: 4},
	{MinLevel: 130, HuntID: 202, HuntName: "Throne of Tafariel", Lure: 4},
	{MinLevel: 130, HuntID: 203, HuntName: "Throne of Apocalypse", Lure: 1},
	{MinLevel: 130, HuntID: 204, HuntName: "Throne of Pumin", Lure: 4},
	{MinLevel: 130, HuntID: 205, HuntName: "Throne of Ashfalor", Lure: 4},
	{MinLevel: 130, HuntID: 206, HuntName: "Throne of Bazir", Lure: 1},
	{MinLevel: 130, HuntID: 207, HuntName: "Ushuriel Seal", Lure: 1},
	{MinLevel: 130, HuntID: 208, HuntName: "Zugurosh Seal", Lure: 1},
	{MinLevel: 130, HuntID: 209, HuntName: "Madareth Seal", Lure: 1},
	{MinLevel: 130, HuntID: 210, HuntName: "Latrivan & Golgordan Seal", Lure: 1},
	{MinLevel: 130, HuntID: 211, HuntName: "Annihilon Seal", Lure: 1},
	{MinLevel: 130, HuntID: 212, HuntName: "Hellgorak Seal", Lure: 1},
	{MinLevel: 130, HuntID: 222, HuntName: "Shadow Nexus", Lure: 4},
	{MinLevel: 130, HuntID: 223, HuntName: "Death Portal", Lure: 4},
	{MinLevel: 130, HuntID: 224, HuntName: "Earth Portal", Lure: 4},
	{MinLevel: 130, HuntID: 225, HuntName: "Energy Portal", Lure: 4},
	{MinLevel: 130, HuntID: 227, HuntName: "Ice Portal", Lure: 4},
	{MinLevel: 130, HuntID: 230, HuntName: "Otherworld Edron", Lure: 4},
	{MinLevel: 130, HuntID: 233, HuntName: "Otherworld Zao", Lure: 4},
}

func getBestiaryFilePath(accountID string) string {
	return filepath.Join("data", fmt.Sprintf("bestiary_%s.json", accountID))
}

// LoadBestiary loads persisted kills and known min levels from disk.
func LoadBestiary(accountID string) (map[string]int, map[int]int) {
	bestiaryMu.Lock()
	defer bestiaryMu.Unlock()

	kills := make(map[string]int)
	minLevels := make(map[int]int)

	filePath := getBestiaryFilePath(accountID)
	b, err := os.ReadFile(filePath)
	if err != nil {
		return kills, minLevels
	}

	var data BestiaryDiskData
	if err := json.Unmarshal(b, &data); err != nil {
		log.Printf("[%s] Erro ao carregar bestiário de %s: %v", accountID, filePath, err)
		return kills, minLevels
	}

	if data.Kills != nil {
		kills = data.Kills
	}
	if data.KnownMinLevels != nil {
		minLevels = data.KnownMinLevels
	}
	log.Printf("[%s] Bestiário carregado do disco: %d criaturas registradas", accountID, len(kills))
	return kills, minLevels
}

// SaveBestiary writes current kills and learned level requirements to disk.
func SaveBestiary(accountID string, kills map[string]int, minLevels map[int]int) {
	bestiaryMu.Lock()
	defer bestiaryMu.Unlock()

	_ = os.MkdirAll("data", 0755)
	filePath := getBestiaryFilePath(accountID)

	data := BestiaryDiskData{
		AccountID:      accountID,
		UpdatedAt:      time.Now().Unix(),
		Kills:          kills,
		KnownMinLevels: minLevels,
	}

	bData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filePath, bData, 0644)
}

// PickNextBestiaryHunt finds the optimal next hunt that has creatures pending bestiary completion.
// It prioritizes hunts unlocked for the current party level, in ascending order of difficulty.
func PickNextBestiaryHunt(currentLevel int, kills map[string]int, knownMinLevels map[int]int) *HuntTier {
	for i := range AllBestiaryHunts {
		tier := &AllBestiaryHunts[i]
		reqLevel := tier.MinLevel
		if knownReq, ok := knownMinLevels[tier.HuntID]; ok && knownReq > reqLevel {
			reqLevel = knownReq
		}

		if currentLevel < reqLevel {
			continue
		}

		def := protocol.FindHuntByID(tier.HuntID)
		if def == nil || len(def.Monsters) == 0 {
			continue
		}

		// Check if any monster in this hunt needs kills
		hasPending := false
		for _, monster := range def.Monsters {
			if kills[monster] < BestiaryTargetKills {
				hasPending = true
				break
			}
		}

		if hasPending {
			return tier
		}
	}
	return nil
}

// BuildBestiaryStatus compiles the full monster list and returns statistics for dashboard.
func BuildBestiaryStatus(kills map[string]int) ([]MonsterKillStat, int, int) {
	// Collect all unique monsters from the full catalog
	monsterSet := make(map[string]bool)
	for _, h := range protocol.Catalog.Hunts {
		for _, m := range h.Monsters {
			monsterSet[m] = true
		}
	}

	completedCount := 0
	totalCount := len(monsterSet)
	var list []MonsterKillStat

	for monster := range monsterSet {
		k := kills[monster]
		done := k >= BestiaryTargetKills
		if done {
			completedCount++
		}
		list = append(list, MonsterKillStat{
			Name:      monster,
			Kills:     k,
			Target:    BestiaryTargetKills,
			Completed: done,
		})
	}

	// Sort: Active farming (kills > 0 and !done) first, then pending (kills == 0), then completed
	sort.Slice(list, func(i, j int) bool {
		m1, m2 := list[i], list[j]
		if m1.Completed != m2.Completed {
			return !m1.Completed // Uncompleted first
		}
		if (m1.Kills > 0) != (m2.Kills > 0) {
			return m1.Kills > 0 // In-progress first
		}
		if m1.Kills != m2.Kills {
			return m1.Kills > m2.Kills // Highest kills first
		}
		return m1.Name < m2.Name
	})

	return list, completedCount, totalCount
}
