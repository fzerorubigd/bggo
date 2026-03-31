package bggo

import "strconv"

// ItemType is the type of item on BGG.
type ItemType string

const (
	BoardGameType          ItemType = "boardgame"
	BoardGameExpansionType ItemType = "boardgameexpansion"
	BoardGameAccessoryType ItemType = "boardgameaccessory"
	RPGItemType            ItemType = "rpgitem"
	VideoGameType          ItemType = "videogame"
)

// Link types for things.
const (
	LinkCategory  = "boardgamecategory"
	LinkMechanic  = "boardgamemechanic"
	LinkFamily    = "boardgamefamily"
	LinkDesigner  = "boardgamedesigner"
	LinkArtist    = "boardgameartist"
	LinkPublisher = "boardgamepublisher"
)

// IDGetter is implemented by any type that carries a BGG item ID.
type IDGetter interface {
	GetID() int64
}

// ExtractIDs returns the IDs from a slice of any type that implements IDGetter.
func ExtractIDs[T IDGetter](items []T) []int64 {
	ids := make([]int64, len(items))
	for i, item := range items {
		ids[i] = item.GetID()
	}
	return ids
}

// Link represents a named reference to another BGG entity.
type Link struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func safeInt(s string) int64 {
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func safeFloat(s string) float64 {
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func safeIntInterface(in any) int64 {
	switch t := in.(type) {
	case string:
		return safeInt(t)
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	default:
		return 0
	}
}
