package block

import (
	"sync"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/recipe"
	"github.com/df-mc/dragonfly/server/world"
)

var registerCrafterTestRecipesOnce sync.Once

func registerCrafterTestRecipes() {
	registerCrafterTestRecipesOnce.Do(func() {
		recipe.Register(recipe.NewShaped([]recipe.Item{
			item.NewStack(item.Diamond{}, 1),
			item.NewStack(item.Bowl{}, 1),
		}, item.NewStack(item.Stick{}, 4), recipe.NewShape(1, 2), "crafting_table"))
		recipe.Register(recipe.NewShapeless([]recipe.Item{
			item.NewStack(item.Bowl{}, 1),
			item.NewStack(item.Stick{}, 1),
		}, item.NewStack(item.Coal{}, 1), "crafting_table"))
	})
}

func TestCrafterStateSupport(t *testing.T) {
	if !crafterStatesSupported() {
		t.Fatal("embedded block states do not support minecraft:crafter")
	}
	if count := len(allCrafters()); count != 48 {
		t.Fatalf("allCrafters returned %d states, want 48", count)
	}
}

func TestCrafterEncodeBlock(t *testing.T) {
	name, props := (Crafter{Orientation: CrafterOrientationNorthUp, Crafting: true, Triggered: true}).EncodeBlock()
	if name != "minecraft:crafter" {
		t.Fatalf("crafter block name = %q, want minecraft:crafter", name)
	}
	if got := props["orientation"]; got != "north_up" {
		t.Fatalf("crafter orientation = %v, want north_up", got)
	}
	if got := props["crafting"]; got != uint8(1) {
		t.Fatalf("crafter crafting = %v, want 1", got)
	}
	if got := props["triggered_bit"]; got != uint8(1) {
		t.Fatalf("crafter triggered_bit = %v, want 1", got)
	}
}

func TestCrafterRegistration(t *testing.T) {
	b := Crafter{Orientation: CrafterOrientationSouthUp, Crafting: true, Triggered: true}
	name, props := b.EncodeBlock()
	got, ok := world.BlockByName(name, props)
	if !ok {
		t.Fatalf("BlockByName(%s, %#v) was not found", name, props)
	}
	if _, ok := got.(Crafter); !ok {
		t.Fatalf("registered block type = %T, want Crafter", got)
	}
}

func TestCrafterSlotMask(t *testing.T) {
	c := NewCrafter()
	if !c.SlotEnabled(4) {
		t.Fatal("slot 4 should be enabled by default")
	}
	c = c.SetSlotEnabled(4, false)
	if c.SlotEnabled(4) {
		t.Fatal("slot 4 should be disabled")
	}
	if c.SlotEnabled(-1) || c.SlotEnabled(9) {
		t.Fatal("out-of-range slots should not be enabled")
	}
	c = c.SetSlotEnabled(4, true)
	if !c.SlotEnabled(4) {
		t.Fatal("slot 4 should be re-enabled")
	}
}

func TestCrafterRedstonePowerUpdate(t *testing.T) {
	after, changed := (Crafter{}).RedstonePowerUpdate(cube.Pos{}, nil, 15)
	if !changed || !after.(Crafter).Triggered {
		t.Fatalf("powered Crafter update = %#v, %v; want triggered change", after, changed)
	}
	after, changed = (Crafter{Triggered: true, Crafting: true}).RedstonePowerUpdate(cube.Pos{}, nil, 0)
	if !changed || after.(Crafter).Triggered || after.(Crafter).Crafting {
		t.Fatalf("unpowered Crafter update = %#v, %v; want untriggered non-crafting change", after, changed)
	}
	_, changed = (Crafter{Triggered: true}).RedstonePowerUpdate(cube.Pos{}, nil, 15)
	if changed {
		t.Fatal("unchanged Crafter power reported a change")
	}
}

func TestCrafterCraftsShapedRecipeOnRisingEdge(t *testing.T) {
	registerCrafterTestRecipes()

	c := NewCrafter()
	_ = c.Inventory(nil, cube.Pos{}).SetItem(0, item.NewStack(item.Diamond{}, 2))
	_ = c.Inventory(nil, cube.Pos{}).SetItem(3, item.NewStack(item.Bowl{}, 1))

	if !c.RedstonePowerAction(cube.Pos{}, nil, 0, 15) {
		t.Fatal("crafter did not craft matching shaped recipe on rising edge")
	}
	diamond, _ := c.Inventory(nil, cube.Pos{}).Item(0)
	if diamond.Count() != 1 {
		t.Fatalf("diamond slot count = %d, want 1", diamond.Count())
	}
	bowl, _ := c.Inventory(nil, cube.Pos{}).Item(3)
	if !bowl.Empty() {
		t.Fatalf("bowl slot = %v, want empty", bowl)
	}
}

func TestCrafterCraftsShapelessRecipe(t *testing.T) {
	registerCrafterTestRecipes()

	c := NewCrafter()
	_ = c.Inventory(nil, cube.Pos{}).SetItem(0, item.NewStack(item.Bowl{}, 1))
	_ = c.Inventory(nil, cube.Pos{}).SetItem(8, item.NewStack(item.Stick{}, 3))

	if !c.RedstonePowerAction(cube.Pos{}, nil, 0, 15) {
		t.Fatal("crafter did not craft matching shapeless recipe")
	}
	stick, _ := c.Inventory(nil, cube.Pos{}).Item(8)
	if stick.Count() != 2 {
		t.Fatalf("stick slot count = %d, want 2", stick.Count())
	}
}

func TestCrafterIgnoresNonRisingEdges(t *testing.T) {
	registerCrafterTestRecipes()

	c := NewCrafter()
	_ = c.Inventory(nil, cube.Pos{}).SetItem(0, item.NewStack(item.Diamond{}, 2))
	_ = c.Inventory(nil, cube.Pos{}).SetItem(3, item.NewStack(item.Bowl{}, 2))

	if c.RedstonePowerAction(cube.Pos{}, nil, 15, 15) {
		t.Fatal("crafter crafted on a steady powered edge")
	}
	if c.RedstonePowerAction(cube.Pos{}, nil, 15, 0) {
		t.Fatal("crafter crafted on a falling edge")
	}
	diamond, _ := c.Inventory(nil, cube.Pos{}).Item(0)
	if diamond.Count() != 2 {
		t.Fatalf("diamond slot count = %d, want 2", diamond.Count())
	}
}

func TestCrafterDisabledSlotBlocksRecipe(t *testing.T) {
	registerCrafterTestRecipes()

	c := NewCrafter().SetSlotEnabled(3, false)
	_ = c.Inventory(nil, cube.Pos{}).SetItem(0, item.NewStack(item.Diamond{}, 1))
	_ = c.Inventory(nil, cube.Pos{}).SetItem(3, item.NewStack(item.Bowl{}, 1))

	if c.RedstonePowerAction(cube.Pos{}, nil, 0, 15) {
		t.Fatal("crafter crafted using a disabled slot")
	}
	diamond, _ := c.Inventory(nil, cube.Pos{}).Item(0)
	if diamond.Count() != 1 {
		t.Fatalf("diamond slot count = %d, want 1", diamond.Count())
	}
}

func TestCrafterComparatorOutput(t *testing.T) {
	c := NewCrafter()
	if got := c.RedstoneComparatorOutput(cube.Pos{}, nil, cube.FaceNorth); got != 0 {
		t.Fatalf("empty crafter comparator output = %d, want 0", got)
	}
	_ = c.Inventory(nil, cube.Pos{}).SetItem(0, item.NewStack(item.Stick{}, 64))
	if got := c.RedstoneComparatorOutput(cube.Pos{}, nil, cube.FaceNorth); got != 2 {
		t.Fatalf("one full crafter slot comparator output = %d, want 2", got)
	}
}

func TestCrafterNBT(t *testing.T) {
	c := NewCrafter().SetSlotEnabled(4, false)
	c.CustomName = "test crafter"
	_ = c.Inventory(nil, cube.Pos{}).SetItem(2, item.NewStack(item.Stick{}, 7))

	data := c.EncodeNBT()
	if got := data["DisabledSlots"]; got != int32(1<<4) {
		t.Fatalf("DisabledSlots = %v, want %d", got, 1<<4)
	}
	decoded := (Crafter{Orientation: CrafterOrientationEastUp, Triggered: true}).DecodeNBT(data).(Crafter)
	if decoded.CustomName != "test crafter" {
		t.Fatalf("CustomName = %q, want test crafter", decoded.CustomName)
	}
	if decoded.SlotEnabled(4) {
		t.Fatal("decoded slot 4 should be disabled")
	}
	stick, _ := decoded.Inventory(nil, cube.Pos{}).Item(2)
	if stick.Count() != 7 {
		t.Fatalf("decoded stick count = %d, want 7", stick.Count())
	}
	if decoded.Orientation != CrafterOrientationEastUp || !decoded.Triggered {
		t.Fatalf("decoded state = %#v, want east_up triggered", decoded)
	}
}

func TestCrafterOrientationFacing(t *testing.T) {
	tests := []struct {
		orientation CrafterOrientation
		face        cube.Face
	}{
		{CrafterOrientationDownNorth, cube.FaceDown},
		{CrafterOrientationUpSouth, cube.FaceUp},
		{CrafterOrientationNorthUp, cube.FaceNorth},
		{CrafterOrientationEastUp, cube.FaceEast},
	}
	for _, test := range tests {
		if got := test.orientation.Facing(); got != test.face {
			t.Fatalf("%v facing = %v, want %v", test.orientation, got, test.face)
		}
	}
}
