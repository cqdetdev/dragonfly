package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/inventory"
)

func TestRedstoneComparatorOutputFromStacks(t *testing.T) {
	tests := []struct {
		name  string
		slots []item.Stack
		want  int
	}{
		{name: "nil", want: 0},
		{name: "empty", slots: []item.Stack{{}}, want: 0},
		{name: "single item", slots: []item.Stack{item.NewStack(item.Stick{}, 1)}, want: 1},
		{name: "half full slot", slots: []item.Stack{item.NewStack(item.Stick{}, 32)}, want: 8},
		{name: "full slot", slots: []item.Stack{item.NewStack(item.Stick{}, 64)}, want: 15},
		{name: "one of two slots full", slots: []item.Stack{item.NewStack(item.Stick{}, 64), {}}, want: 8},
		{name: "unstackable item fills slot", slots: []item.Stack{item.NewStack(item.MusicDisc{}, 1)}, want: 15},
		{name: "overfull stack clamps", slots: []item.Stack{item.NewStack(item.Stick{}, 128)}, want: 15},
	}
	for _, tt := range tests {
		if got := redstoneComparatorOutputFromStacks(tt.slots); got != tt.want {
			t.Fatalf("%s output = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestRedstoneComparatorOutputFromInventory(t *testing.T) {
	if got := redstoneComparatorOutputFromInventory(nil); got != 0 {
		t.Fatalf("nil inventory output = %d, want 0", got)
	}

	inv := inventory.New(2, nil)
	_ = inv.SetItem(0, item.NewStack(item.Stick{}, 64))
	if got := redstoneComparatorOutputFromInventory(inv); got != 8 {
		t.Fatalf("half-full inventory output = %d, want 8", got)
	}
}

func TestRedstoneComparatorContainerBlockOutputs(t *testing.T) {
	if got := (Chest{}).RedstoneComparatorOutput(cube.Pos{}, nil, cube.FaceNorth); got != 0 {
		t.Fatalf("zero-value chest output = %d, want 0", got)
	}
	if got := (Barrel{}).RedstoneComparatorOutput(cube.Pos{}, nil, cube.FaceNorth); got != 0 {
		t.Fatalf("zero-value barrel output = %d, want 0", got)
	}
	if got := (Hopper{}).RedstoneComparatorOutput(cube.Pos{}, nil, cube.FaceNorth); got != 0 {
		t.Fatalf("zero-value hopper output = %d, want 0", got)
	}
	if got := (BrewingStand{}).RedstoneComparatorOutput(cube.Pos{}, nil, cube.FaceNorth); got != 0 {
		t.Fatalf("zero-value brewing stand output = %d, want 0", got)
	}
	if got := (Furnace{}).RedstoneComparatorOutput(cube.Pos{}, nil, cube.FaceNorth); got != 0 {
		t.Fatalf("zero-value furnace output = %d, want 0", got)
	}

	chest := NewChest()
	_ = chest.Inventory(nil, cube.Pos{}).SetItem(0, item.NewStack(item.Stick{}, 64))
	if got := chest.RedstoneComparatorOutput(cube.Pos{}, nil, cube.FaceNorth); got != 1 {
		t.Fatalf("single-slot chest output = %d, want 1", got)
	}

	barrel := NewBarrel()
	_ = barrel.Inventory(nil, cube.Pos{}).SetItem(0, item.NewStack(item.Stick{}, 64))
	if got := barrel.RedstoneComparatorOutput(cube.Pos{}, nil, cube.FaceNorth); got != 1 {
		t.Fatalf("single-slot barrel output = %d, want 1", got)
	}

	hopper := NewHopper()
	for slot := 0; slot < hopper.Inventory(nil, cube.Pos{}).Size(); slot++ {
		_ = hopper.Inventory(nil, cube.Pos{}).SetItem(slot, item.NewStack(item.Stick{}, 64))
	}
	if got := hopper.RedstoneComparatorOutput(cube.Pos{}, nil, cube.FaceNorth); got != 15 {
		t.Fatalf("full hopper output = %d, want 15", got)
	}

	stand := NewBrewingStand()
	_ = stand.Inventory(nil, cube.Pos{}).SetItem(0, item.NewStack(item.Stick{}, 64))
	if got := stand.RedstoneComparatorOutput(cube.Pos{}, nil, cube.FaceNorth); got != 3 {
		t.Fatalf("single-slot brewing stand output = %d, want 3", got)
	}

	furnace := NewFurnace(cube.North)
	for slot := 0; slot < furnace.Inventory(nil, cube.Pos{}).Size(); slot++ {
		_ = furnace.Inventory(nil, cube.Pos{}).SetItem(slot, item.NewStack(item.Stick{}, 64))
	}
	if got := furnace.RedstoneComparatorOutput(cube.Pos{}, nil, cube.FaceNorth); got != 15 {
		t.Fatalf("full furnace output = %d, want 15", got)
	}
}

func TestRedstoneComparatorNonContainerBlockOutputs(t *testing.T) {
	tests := []struct {
		name string
		out  int
		want int
	}{
		{name: "full cake", out: (Cake{}).RedstoneComparatorOutput(cube.Pos{}, nil, cube.FaceNorth), want: 14},
		{name: "partial cake", out: (Cake{Bites: 3}).RedstoneComparatorOutput(cube.Pos{}, nil, cube.FaceNorth), want: 8},
		{name: "last cake slice", out: (Cake{Bites: 6}).RedstoneComparatorOutput(cube.Pos{}, nil, cube.FaceNorth), want: 2},
		{name: "empty composter", out: (Composter{}).RedstoneComparatorOutput(cube.Pos{}, nil, cube.FaceNorth), want: 0},
		{name: "ready composter", out: (Composter{Level: 8}).RedstoneComparatorOutput(cube.Pos{}, nil, cube.FaceNorth), want: 8},
		{name: "empty jukebox", out: (Jukebox{}).RedstoneComparatorOutput(cube.Pos{}, nil, cube.FaceNorth), want: 0},
		{name: "occupied jukebox", out: (Jukebox{Item: item.NewStack(item.MusicDisc{}, 1)}).RedstoneComparatorOutput(cube.Pos{}, nil, cube.FaceNorth), want: 15},
	}
	for _, tt := range tests {
		if tt.out != tt.want {
			t.Fatalf("%s output = %d, want %d", tt.name, tt.out, tt.want)
		}
	}
}
