package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
)

func TestTrappedChestRedstonePower(t *testing.T) {
	chest := NewTrappedChest()
	if power := chest.RedstonePower(cube.Pos{}, nil, cube.FaceUp); power != 0 {
		t.Fatalf("empty trapped chest power = %d, want 0", power)
	}
	chest.viewers[nil] = struct{}{}
	if power := chest.RedstonePower(cube.Pos{}, nil, cube.FaceUp); power != 1 {
		t.Fatalf("open trapped chest power = %d, want 1", power)
	}
	if power := NewChest().RedstonePower(cube.Pos{}, nil, cube.FaceUp); power != 0 {
		t.Fatalf("regular chest power = %d, want 0", power)
	}
}

func TestTrappedChestEncode(t *testing.T) {
	name, _ := (Chest{Trapped: true}).EncodeBlock()
	if name != "minecraft:trapped_chest" {
		t.Fatalf("trapped chest block name = %q, want minecraft:trapped_chest", name)
	}
	itemName, _ := (Chest{Trapped: true}).EncodeItem()
	if itemName != "minecraft:trapped_chest" {
		t.Fatalf("trapped chest item name = %q, want minecraft:trapped_chest", itemName)
	}
}

func TestHopperRedstoneConsumer(t *testing.T) {
	if after, changed := (Hopper{}).RedstonePowerUpdate(cube.Pos{}, nil, 15); !changed || !after.(Hopper).Powered {
		t.Fatalf("powered Hopper update = %#v, %v; want powered change", after, changed)
	}
}
