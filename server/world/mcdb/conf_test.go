package mcdb

import (
	"path/filepath"
	"testing"

	"github.com/df-mc/dragonfly/server/world/mcdb/leveldat"
)

// TestOpenRespawnBlocksExplodeDefaults verifies that missing gamerules use the vanilla default while explicit values
// are preserved.
func TestOpenRespawnBlocksExplodeDefaults(t *testing.T) {
	tests := []struct {
		name   string
		fields map[string]any
		want   bool
	}{
		{name: "missing uses vanilla default", fields: map[string]any{"LevelName": "Legacy World"}, want: true},
		{name: "explicit false is preserved", fields: map[string]any{"respawnblocksexplode": false}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			var dat leveldat.LevelDat
			if err := dat.Marshal(tt.fields); err != nil {
				t.Fatalf("marshal level.dat: %v", err)
			}
			if err := dat.WriteFile(filepath.Join(dir, "level.dat")); err != nil {
				t.Fatalf("write level.dat: %v", err)
			}

			db, err := Open(dir)
			if err != nil {
				t.Fatalf("open world: %v", err)
			}
			t.Cleanup(func() { _ = db.Close() })
			if got := db.Settings().RespawnBlocksExplode; got != tt.want {
				t.Fatalf("RespawnBlocksExplode = %v, want %v", got, tt.want)
			}
		})
	}
}
