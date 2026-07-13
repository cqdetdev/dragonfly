package block

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/sound"
	"github.com/go-gl/mathgl/mgl64"
)

// redstoneComparatorReadable is implemented by blocks that expose an analog
// signal to a comparator behind them, such as containers exposing their fill
// level.
type redstoneComparatorReadable interface {
	// RedstoneComparatorOutput returns the analog power level, from 0 to 15,
	// read by a comparator facing the direction passed.
	RedstoneComparatorOutput(pos cube.Pos, tx *world.Tx, facing cube.Face) int
}

// Comparator is a directional analog redstone component that compares or
// subtracts side input from rear input.
type Comparator struct {
	empty
	transparent
	sourceWaterDisplacer

	// Facing is the output direction of the comparator.
	Facing cube.Direction
	// Subtract is true when the comparator is in subtract mode.
	Subtract bool
	// Powered is true if the comparator is currently emitting power.
	Powered bool
}

// UseOnBlock places the comparator on a solid surface.
func (c Comparator) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	pos, _, used := firstReplaceable(tx, pos, face, c)
	if !used || !redstoneFloorComponentSupported(tx, pos) {
		return false
	}
	if user != nil {
		c.Facing = user.Rotation().Direction()
	}
	place(tx, pos, c, user, ctx)
	return placed(ctx)
}

// Activate toggles between compare and subtract mode.
func (c Comparator) Activate(pos cube.Pos, _ cube.Face, tx *world.Tx, _ item.User, _ *item.UseContext) bool {
	c.Subtract = !c.Subtract
	tx.SetBlock(pos, c, nil)
	tx.PlaySound(pos.Vec3Centre(), sound.Click{})
	return true
}

// NeighbourUpdateTick breaks the comparator if its supporting block is removed.
func (c Comparator) NeighbourUpdateTick(pos, _ cube.Pos, tx *world.Tx) {
	if !redstoneFloorComponentSupported(tx, pos) {
		breakBlock(c, pos, tx)
	}
}

// RedstonePower emits the comparator's analog output from the output side.
func (c Comparator) RedstonePower(pos cube.Pos, tx *world.Tx, face cube.Face) int {
	if face != c.Facing.Face() {
		return 0
	}
	if tx == nil {
		if c.Powered {
			return 15
		}
		return 0
	}
	return c.outputPower(pos, tx)
}

// RedstoneStrongPower strongly powers the block in front of the comparator.
func (c Comparator) RedstoneStrongPower(pos cube.Pos, tx *world.Tx, face cube.Face) int {
	return c.RedstonePower(pos, tx, face)
}

// RedstonePowerUpdate updates the lit state to match the comparator output.
func (c Comparator) RedstonePowerUpdate(pos cube.Pos, tx *world.Tx, _ int) (world.Block, bool) {
	if tx == nil {
		return c, false
	}
	powered := c.outputPower(pos, tx) > 0
	if c.Powered == powered {
		return c, false
	}
	c.Powered = powered
	return c, true
}

// outputPower computes the comparator's analog output from its rear input, a
// comparator-readable block behind it and its side inputs.
func (c Comparator) outputPower(pos cube.Pos, tx *world.Tx) int {
	rearFace := c.Facing.Opposite().Face()
	rearPower := tx.RedstonePowerFrom(pos, rearFace)
	rearPos := pos.Side(rearFace)
	if b, ok := tx.BlockLoaded(rearPos); ok {
		if readable, ok := b.(redstoneComparatorReadable); ok {
			rearPower = max(rearPower, world.ClampRedstonePower(readable.RedstoneComparatorOutput(rearPos, tx, c.Facing.Face())))
		}
	}

	sidePower := max(tx.RedstonePowerFrom(pos, c.Facing.RotateLeft().Face()), tx.RedstonePowerFrom(pos, c.Facing.RotateRight().Face()))
	if c.Subtract {
		return world.ClampRedstonePower(rearPower - sidePower)
	}
	if rearPower >= sidePower {
		return world.ClampRedstonePower(rearPower)
	}
	return 0
}

// BreakInfo ...
func (c Comparator) BreakInfo() BreakInfo {
	return newBreakInfo(0, alwaysHarvestable, nothingEffective, oneOf(c))
}

// SideClosed ...
func (Comparator) SideClosed(cube.Pos, cube.Pos, *world.Tx) bool {
	return false
}

// EncodeItem ...
func (Comparator) EncodeItem() (name string, meta int16) {
	return "minecraft:comparator", 0
}

// EncodeBlock ...
func (c Comparator) EncodeBlock() (string, map[string]any) {
	name := "minecraft:unpowered_comparator"
	if c.Powered {
		name = "minecraft:powered_comparator"
	}
	return name, map[string]any{
		"minecraft:cardinal_direction": c.Facing.Opposite().String(),
		"output_lit_bit":               boolByte(c.Powered),
		"output_subtract_bit":          boolByte(c.Subtract),
	}
}

// allComparators ...
func allComparators() (comparators []world.Block) {
	for _, facing := range cube.Directions() {
		for _, subtract := range []bool{false, true} {
			comparators = append(comparators, Comparator{Facing: facing, Subtract: subtract}, Comparator{Facing: facing, Subtract: subtract, Powered: true})
		}
	}
	return
}
