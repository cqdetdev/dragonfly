package block

import (
	"math/rand/v2"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// Observer is a block that emits a short redstone pulse when the block in
// front of it changes.
type Observer struct {
	solid
	sourceWaterDisplacer

	// Facing is the face observed by the observer.
	Facing cube.Face
	// Powered is true while the observer is emitting its pulse.
	Powered bool
}

// UseOnBlock places the observer with its observing side facing the user.
func (o Observer) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	pos, face, used := firstReplaceable(tx, pos, face, o)
	if !used {
		return false
	}
	if user != nil {
		o.Facing = calculateFace(user, pos).Opposite()
	} else {
		o.Facing = face.Opposite()
	}
	place(tx, pos, o, user, ctx)
	return placed(ctx)
}

// NeighbourUpdateTick starts an observer pulse when its observed block changes.
func (o Observer) NeighbourUpdateTick(pos, changedNeighbour cube.Pos, tx *world.Tx) {
	if o.Powered || changedNeighbour != pos.Side(o.Facing) {
		return
	}
	o.Powered = true
	tx.SetBlock(pos, o, nil)
	tx.ScheduleBlockUpdate(pos, o, redstoneTicks(1))
}

// ScheduledTick ends an observer pulse.
func (o Observer) ScheduledTick(pos cube.Pos, tx *world.Tx, _ *rand.Rand) {
	if !o.Powered {
		return
	}
	o.Powered = false
	tx.SetBlock(pos, o, nil)
}

// RedstonePower emits maximum power from the back while pulsing.
func (o Observer) RedstonePower(_ cube.Pos, _ *world.Tx, face cube.Face) int {
	if o.Powered && face == o.Facing.Opposite() {
		return 15
	}
	return 0
}

// BreakInfo ...
func (o Observer) BreakInfo() BreakInfo {
	return newBreakInfo(3, pickaxeHarvestable, pickaxeEffective, oneOf(o))
}

// EncodeItem ...
func (Observer) EncodeItem() (name string, meta int16) {
	return "minecraft:observer", 0
}

// EncodeBlock ...
func (o Observer) EncodeBlock() (string, map[string]any) {
	return "minecraft:observer", map[string]any{"minecraft:facing_direction": o.Facing.String(), "powered_bit": boolByte(o.Powered)}
}

// allObservers ...
func allObservers() (observers []world.Block) {
	for _, facing := range cube.Faces() {
		observers = append(observers, Observer{Facing: facing}, Observer{Facing: facing, Powered: true})
	}
	return
}
