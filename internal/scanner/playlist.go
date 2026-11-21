package scanner

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// PlaylistRecord is what Stargazer remembers about the playlist it created. It is
// machine-owned: written by "playlist sync", never by hand.
type PlaylistRecord struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	LastSync string `json:"last_sync,omitempty"`
	// Unavailable lists video ids YouTube refused to add (deleted, private, blocked).
	// They are left out of the wanted list so a dead link never blocks the sync or
	// costs quota again; remove an id here to try it again.
	Unavailable []string `json:"unavailable,omitempty"`
}

// LoadPlaylistRecord reads the record. A missing file is not an error: it means no
// playlist has been created yet, and an empty record is returned.
func LoadPlaylistRecord(path string) (PlaylistRecord, error) {
	var rec PlaylistRecord
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return rec, nil
		}
		return rec, fmt.Errorf("unable to read playlist record: %w", err)
	}
	if err := json.Unmarshal(b, &rec); err != nil {
		return rec, fmt.Errorf("playlist record %s is not valid JSON: %w", path, err)
	}
	return rec, nil
}

// SavePlaylistRecord writes the record atomically with owner-only permissions.
func SavePlaylistRecord(path string, rec PlaylistRecord) error {
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp")
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return fmt.Errorf("unable to write playlist record: %w", err)
	}
	return os.Rename(tmp, path)
}
