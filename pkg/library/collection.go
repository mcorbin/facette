package library

import (
	"strings"

	"github.com/facette/facette/pkg/config"
)

// Collection represents a collection of graphs.
type Collection struct {
	Item
	Entries  []*CollectionEntry `json:"entries"`
	Parent   *Collection        `json:"-"`
	ParentID string             `json:"parent"`
	Children []*Collection      `json:"-"`
}

// CollectionEntry represents a collection entry.
type CollectionEntry struct {
	ID      string                 `json:"id"`
	Options map[string]interface{} `json:"options"`
}

// FilterCollection filters collection entries by graphs titles and enable state.
func (library *Library) FilterCollection(collection *Collection, filter string) *Collection {
	collectionTemp := &Collection{}
	*collectionTemp = *collection
	collectionTemp.Entries = nil

	for _, entry := range collection.Entries {
		if enabled, err := config.GetBool(entry.Options, "enabled", false); err != nil || !enabled {
			continue
		} else if filter != "" {
			if title, err := config.GetString(entry.Options, "title", false); err != nil ||
				!strings.Contains(strings.ToLower(title), strings.ToLower(filter)) {
				continue
			}
		}

		collectionTemp.Entries = append(collectionTemp.Entries, entry)
	}

	return collectionTemp
}
