package block

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/inventory"
	"github.com/df-mc/dragonfly/server/world"
)

var (
	_ world.RedstoneComparatorReadable = Chest{}
	_ world.RedstoneComparatorReadable = Barrel{}
	_ world.RedstoneComparatorReadable = Hopper{}
	_ world.RedstoneComparatorReadable = BrewingStand{}
	_ world.RedstoneComparatorReadable = Furnace{}
	_ world.RedstoneComparatorReadable = BlastFurnace{}
	_ world.RedstoneComparatorReadable = Smoker{}
	_ world.RedstoneComparatorReadable = Cake{}
	_ world.RedstoneComparatorReadable = Composter{}
	_ world.RedstoneComparatorReadable = Jukebox{}
)

// RedstoneComparatorOutput returns the analog output for the chest's contents.
func (c Chest) RedstoneComparatorOutput(pos cube.Pos, tx *world.Tx, _ cube.Face) int {
	if c.pairInv != nil {
		return redstoneComparatorOutputFromInventory(c.pairInv)
	}
	if tx == nil || !c.paired {
		return redstoneComparatorOutputFromInventory(c.inventory)
	}
	pair, ok := tx.BlockLoaded(c.pairPos(pos))
	if !ok {
		return redstoneComparatorOutputFromInventory(c.inventory)
	}
	pairChest, ok := pair.(Chest)
	if !ok || pairChest.Facing != c.Facing || pairChest.Trapped != c.Trapped {
		return redstoneComparatorOutputFromInventory(c.inventory)
	}
	return redstoneComparatorOutputFromStacks(append(c.inventorySlots(), pairChest.inventorySlots()...))
}

// RedstoneComparatorOutput returns the analog output for the barrel's contents.
func (b Barrel) RedstoneComparatorOutput(cube.Pos, *world.Tx, cube.Face) int {
	return redstoneComparatorOutputFromInventory(b.inventory)
}

// RedstoneComparatorOutput returns the analog output for the hopper's contents.
func (h Hopper) RedstoneComparatorOutput(cube.Pos, *world.Tx, cube.Face) int {
	return redstoneComparatorOutputFromInventory(h.inventory)
}

// RedstoneComparatorOutput returns the analog output for the cake's remaining bites.
func (c Cake) RedstoneComparatorOutput(cube.Pos, *world.Tx, cube.Face) int {
	bites := max(0, min(c.Bites, 7))
	return (7 - bites) * 2
}

// RedstoneComparatorOutput returns the analog output for the composter's fill level.
func (c Composter) RedstoneComparatorOutput(cube.Pos, *world.Tx, cube.Face) int {
	return max(0, min(c.Level, 8))
}

// RedstoneComparatorOutput returns maximum analog output while the jukebox is occupied.
func (j Jukebox) RedstoneComparatorOutput(cube.Pos, *world.Tx, cube.Face) int {
	if j.Item.Empty() {
		return 0
	}
	return 15
}

// RedstoneComparatorOutput returns the analog output for the brewer's contents.
func (b *brewer) RedstoneComparatorOutput(cube.Pos, *world.Tx, cube.Face) int {
	if b == nil {
		return 0
	}
	return redstoneComparatorOutputFromInventory(b.inventory)
}

// RedstoneComparatorOutput returns the analog output for the smelter's contents.
func (s *smelter) RedstoneComparatorOutput(cube.Pos, *world.Tx, cube.Face) int {
	if s == nil {
		return 0
	}
	return redstoneComparatorOutputFromInventory(s.inventory)
}

func redstoneComparatorOutputFromInventory(inv *inventory.Inventory) int {
	if inv == nil {
		return 0
	}
	return redstoneComparatorOutputFromStacks(inv.Slots())
}

func (c Chest) inventorySlots() []item.Stack {
	if c.inventory == nil {
		return nil
	}
	return c.inventory.Slots()
}

func redstoneComparatorOutputFromStacks(slots []item.Stack) int {
	if len(slots) == 0 {
		return 0
	}
	occupied, fullness := 0, 0.0
	for _, stack := range slots {
		if stack.Empty() {
			continue
		}
		maxCount := stack.MaxCount()
		if maxCount <= 0 {
			continue
		}
		fullness += float64(min(stack.Count(), maxCount)) / float64(maxCount)
		occupied++
	}
	if occupied == 0 {
		return 0
	}
	return max(0, min(1+int(fullness/float64(len(slots))*14), 15))
}
