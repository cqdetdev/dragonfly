package block

import (
	"math/rand/v2"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/sound"
	"github.com/go-gl/mathgl/mgl64"
)

// Repeater is a directional redstone component that delays and refreshes a
// digital signal.
type Repeater struct {
	empty
	transparent
	sourceWaterDisplacer

	// Facing is the output direction of the repeater.
	Facing cube.Direction
	// Delay is the repeater delay setting, from 0 to 3, representing 1 to 4
	// redstone ticks.
	Delay int
	// Powered is true if the repeater is currently emitting power.
	Powered bool
}

// UseOnBlock places the repeater on a solid surface.
func (r Repeater) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	pos, _, used := firstReplaceable(tx, pos, face, r)
	if !used || !redstoneFloorComponentSupported(tx, pos) {
		return false
	}
	if user != nil {
		r.Facing = user.Rotation().Direction()
	}
	place(tx, pos, r, user, ctx)
	return placed(ctx)
}

// Activate cycles the repeater delay.
func (r Repeater) Activate(pos cube.Pos, _ cube.Face, tx *world.Tx, _ item.User, _ *item.UseContext) bool {
	r.Delay = (r.delay() + 1) % 4
	tx.SetBlock(pos, r, nil)
	tx.PlaySound(pos.Vec3Centre(), sound.Click{})
	return true
}

// NeighbourUpdateTick breaks the repeater if its supporting block is removed.
func (r Repeater) NeighbourUpdateTick(pos, _ cube.Pos, tx *world.Tx) {
	if !redstoneFloorComponentSupported(tx, pos) {
		breakBlock(r, pos, tx)
	}
}

// ScheduledTick applies the delayed input state.
func (r Repeater) ScheduledTick(pos cube.Pos, tx *world.Tx, _ *rand.Rand) {
	if r.locked(pos, tx) {
		return
	}
	powered := r.rearPowered(pos, tx)
	if r.Powered == powered {
		return
	}
	r.Powered = powered
	tx.SetBlock(pos, r, nil)
}

// RedstonePower emits refreshed power from the repeater output side.
func (r Repeater) RedstonePower(_ cube.Pos, _ *world.Tx, face cube.Face) int {
	if r.Powered && face == r.Facing.Face() {
		return 15
	}
	return 0
}

// RedstoneStrongPower strongly powers the block in front of the repeater.
func (r Repeater) RedstoneStrongPower(pos cube.Pos, tx *world.Tx, face cube.Face) int {
	return r.RedstonePower(pos, tx, face)
}

// RedstonePowerUpdate schedules a delayed state change when the rear input
// changes, unless the repeater is locked by a powered side input.
func (r Repeater) RedstonePowerUpdate(pos cube.Pos, tx *world.Tx, _ int) (world.Block, bool) {
	if tx == nil || r.locked(pos, tx) || r.Powered == r.rearPowered(pos, tx) {
		return r, false
	}
	tx.ScheduleBlockUpdate(pos, r, redstoneTicks(r.delay()+1))
	return r, false
}

// rearPowered reports whether the repeater's input side receives power.
func (r Repeater) rearPowered(pos cube.Pos, tx *world.Tx) bool {
	return tx.RedstonePowerFrom(pos, r.Facing.Opposite().Face()) > 0
}

// locked reports whether a powered side input locks the repeater's state.
func (r Repeater) locked(pos cube.Pos, tx *world.Tx) bool {
	return tx.RedstonePowerFrom(pos, r.Facing.RotateLeft().Face()) > 0 ||
		tx.RedstonePowerFrom(pos, r.Facing.RotateRight().Face()) > 0
}

// delay returns the delay setting clamped to its valid range.
func (r Repeater) delay() int {
	return max(0, min(r.Delay, 3))
}

// BreakInfo ...
func (r Repeater) BreakInfo() BreakInfo {
	return newBreakInfo(0, alwaysHarvestable, nothingEffective, oneOf(r))
}

// SideClosed ...
func (Repeater) SideClosed(cube.Pos, cube.Pos, *world.Tx) bool {
	return false
}

// EncodeItem ...
func (Repeater) EncodeItem() (name string, meta int16) {
	return "minecraft:repeater", 0
}

// EncodeBlock ...
func (r Repeater) EncodeBlock() (string, map[string]any) {
	name := "minecraft:unpowered_repeater"
	if r.Powered {
		name = "minecraft:powered_repeater"
	}
	delay := max(0, min(r.Delay, 3))
	return name, map[string]any{
		"minecraft:cardinal_direction": r.Facing.Opposite().String(),
		"repeater_delay":               int32(delay),
	}
}

// allRepeaters ...
func allRepeaters() (repeaters []world.Block) {
	for _, facing := range cube.Directions() {
		for delay := 0; delay <= 3; delay++ {
			repeaters = append(repeaters, Repeater{Facing: facing, Delay: delay}, Repeater{Facing: facing, Delay: delay, Powered: true})
		}
	}
	return
}
