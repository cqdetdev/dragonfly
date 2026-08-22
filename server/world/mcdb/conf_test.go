package mcdb

import (
	"path/filepath"
	"testing"

	"github.com/df-mc/dragonfly/server/world/mcdb/leveldat"
)

// TestOpenDefaultsMissingRespawnBlocksExplode verifies that worlds created before the gamerule was persisted retain
// the vanilla default when opened.
func TestOpenDefaultsMissingRespawnBlocksExplode(t *testing.T) {
	dir := t.TempDir()
	var dat leveldat.LevelDat
	if err := dat.Marshal(map[string]any{"LevelName": "Legacy World"}); err != nil {
		t.Fatalf("marshal legacy level.dat: %v", err)
	}
	if err := dat.WriteFile(filepath.Join(dir, "level.dat")); err != nil {
		t.Fatalf("write legacy level.dat: %v", err)
	}

	db, err := Open(dir)
	if err != nil {
		t.Fatalf("open legacy world: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if !db.Settings().RespawnBlocksExplode {
		t.Fatal("RespawnBlocksExplode = false for a level.dat without the gamerule, want vanilla default true")
	}
}
