package block_test

import (
	"context"
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/google/uuid"
)

// TestRespawnAnchorWaterSuppressesBlocksButNotEntities verifies that adjacent water prevents block destruction without
// making entities on the unobstructed side immune to the explosion.
func TestRespawnAnchorWaterSuppressesBlocksButNotEntities(t *testing.T) {
	w := world.Config{Synchronous: true}.New()
	t.Cleanup(func() { _ = w.Close() })

	anchorPos := cube.Pos{0, 1, 0}
	id := uuid.New()
	handle := world.EntitySpawnOpts{Position: mgl64.Vec3{-1.5, 1.5, 0.5}, ID: id}.New(player.Type, player.Config{
		UUID: id, Health: 100, MaxHealth: 100,
	})

	_, err := world.Call(context.Background(), w, func(tx *world.Tx) (struct{}, error) {
		p := tx.AddEntity(handle).(*player.Player)
		anchor := block.RespawnAnchor{Charges: 1}
		tx.SetBlock(anchorPos, anchor, nil)
		tx.SetBlock(anchorPos.Side(cube.FaceNorth), block.Stone{}, nil)
		tx.SetLiquid(anchorPos.Side(cube.FaceEast), block.Water{Still: true, Depth: 8})

		before := p.Health()
		if !anchor.Activate(anchorPos, cube.FaceUp, tx, p, new(item.UseContext)) {
			t.Fatal("charged respawn anchor was not activated")
		}
		if got := p.Health(); got >= before {
			t.Fatalf("player health after water-adjacent anchor explosion = %v, want less than %v", got, before)
		}
		if got := tx.Block(anchorPos.Side(cube.FaceNorth)); got != (block.Stone{}) {
			t.Fatalf("block beside water-adjacent anchor = %T, want block.Stone", got)
		}
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatalf("run anchor explosion: %v", err)
	}
}
