package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ── shared helpers ──────────────────────────────────────────────────────────

// writeJSON marshals v to JSON and writes it to path (creating parent dirs as needed).
func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// loadFallbackCountmeHistory reads week_records from the committed src/data/countme.json
// and reconstructs a HistoryStore from it. Used when both the cache and the live CSV fetch
// are unavailable (e.g. CI cold-start with no read:packages scope or rate-limited endpoint).
