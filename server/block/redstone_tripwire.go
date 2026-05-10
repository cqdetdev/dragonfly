package block

import (
	"math/rand/v2"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

const tripwireReleaseTicks = 5

// EntityInside powers tripwire when an entity passes through it.
func (s String) EntityInside(pos cube.Pos, tx *world.Tx, _ world.Entity) {
	if s.Disarmed {
		return
	}
	if !s.Powered {
		s.Powered = true
		tx.SetBlock(pos, s, nil)
	}
	tx.ScheduleBlockUpdate(pos, s, redstoneTicks(tripwireReleaseTicks))
}

// ScheduledTick releases a tripwire if no entity refreshed it.
func (s String) ScheduledTick(pos cube.Pos, tx *world.Tx, _ *rand.Rand) {
	if !s.Powered {
		return
	}
	s.Powered = false
	tx.SetBlock(pos, s, nil)
}

// RedstonePower returns maximum power while the tripwire is activated.
func (s String) RedstonePower(cube.Pos, *world.Tx, cube.Face) int {
	if s.Powered && !s.Disarmed {
		return 15
	}
	return 0
}

// TripwireHook is a wall-mounted redstone source driven by tripwire.
type TripwireHook struct {
	empty
	transparent
	sourceWaterDisplacer

	// Direction is the Bedrock horizontal direction state.
	Direction int
	// Attached is true if the hook is connected to a valid tripwire line.
	Attached bool
	// Powered is true if its tripwire line is activated.
	Powered bool
}

// UseOnBlock places a tripwire hook on a horizontal face.
func (h TripwireHook) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	pos, face, used := firstReplaceable(tx, pos, face, h)
	if !used || face == cube.FaceUp || face == cube.FaceDown || !redstoneAttachmentSupported(tx, pos, face) {
		return false
	}
	h.Direction = tripwireHookDirection(face.Direction())
	place(tx, pos, h, user, ctx)
	return placed(ctx)
}

// NeighbourUpdateTick breaks the hook if its supporting block is removed.
func (h TripwireHook) NeighbourUpdateTick(pos, _ cube.Pos, tx *world.Tx) {
	if !redstoneAttachmentSupported(tx, pos, tripwireHookFace(h.Direction)) {
		breakBlock(h, pos, tx)
	}
}

// RedstonePower returns maximum power while the hook is powered.
func (h TripwireHook) RedstonePower(cube.Pos, *world.Tx, cube.Face) int {
	if h.Powered {
		return 15
	}
	return 0
}

// RedstonePowerUpdate updates the hook powered state from adjacent tripwire power.
func (h TripwireHook) RedstonePowerUpdate(_ cube.Pos, _ *world.Tx, power int) (world.Block, bool) {
	powered := power > 0
	if h.Powered == powered {
		return h, false
	}
	h.Powered = powered
	return h, true
}

// BreakInfo ...
func (h TripwireHook) BreakInfo() BreakInfo {
	return newBreakInfo(0, alwaysHarvestable, nothingEffective, oneOf(h))
}

// SideClosed ...
func (TripwireHook) SideClosed(cube.Pos, cube.Pos, *world.Tx) bool {
	return false
}

// EncodeItem ...
func (TripwireHook) EncodeItem() (name string, meta int16) {
	return "minecraft:tripwire_hook", 0
}

// EncodeBlock ...
func (h TripwireHook) EncodeBlock() (string, map[string]any) {
	return "minecraft:tripwire_hook", map[string]any{
		"direction":    int32(max(0, min(h.Direction, 3))),
		"attached_bit": boolByte(h.Attached),
		"powered_bit":  boolByte(h.Powered),
	}
}

func allTripwireHooks() (hooks []world.Block) {
	for direction := 0; direction < 4; direction++ {
		for _, attached := range []bool{false, true} {
			hooks = append(hooks, TripwireHook{Direction: direction, Attached: attached}, TripwireHook{Direction: direction, Attached: attached, Powered: true})
		}
	}
	return
}

func tripwireHookDirection(direction cube.Direction) int {
	switch direction {
	case cube.East:
		return 3
	case cube.South:
		return 2
	case cube.West:
		return 1
	default:
		return 0
	}
}

func tripwireHookFace(direction int) cube.Face {
	switch direction {
	case 1:
		return cube.FaceWest
	case 2:
		return cube.FaceSouth
	case 3:
		return cube.FaceEast
	default:
		return cube.FaceNorth
	}
}
