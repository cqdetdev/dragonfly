package block

import (
	"math/rand/v2"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/sound"
	"github.com/go-gl/mathgl/mgl64"
)

// Repeater is a directional redstone component that delays and refreshes a digital signal.
type Repeater struct {
	empty
	transparent
	sourceWaterDisplacer

	// Facing is the output direction of the repeater.
	Facing cube.Direction
	// Delay is the repeater delay setting, from 0 to 3, representing 1 to 4 redstone ticks.
	Delay int
	// Powered is true if the repeater is currently emitting power.
	Powered bool
}

// UseOnBlock places a repeater on a replaceable block.
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

// RedstonePowerUpdate schedules a delayed state change when the rear input changes.
func (r Repeater) RedstonePowerUpdate(pos cube.Pos, tx *world.Tx, _ int) (world.Block, bool) {
	if tx == nil || r.locked(pos, tx) || r.Powered == r.rearPowered(pos, tx) {
		return r, false
	}
	tx.ScheduleBlockUpdate(pos, r, redstoneTicks(r.delay()+1))
	return r, false
}

func (r Repeater) rearPowered(pos cube.Pos, tx *world.Tx) bool {
	return tx.RedstonePowerFrom(pos, r.Facing.Opposite().Face()) > 0
}

func (r Repeater) locked(pos cube.Pos, tx *world.Tx) bool {
	return tx.RedstonePowerFrom(pos, r.Facing.RotateLeft().Face()) > 0 ||
		tx.RedstonePowerFrom(pos, r.Facing.RotateRight().Face()) > 0
}

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

func allRepeaters() (repeaters []world.Block) {
	for _, facing := range cube.Directions() {
		for delay := 0; delay <= 3; delay++ {
			repeaters = append(repeaters, Repeater{Facing: facing, Delay: delay}, Repeater{Facing: facing, Delay: delay, Powered: true})
		}
	}
	return
}

// Comparator is a directional analog redstone component that compares or subtracts side input from rear input.
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

// UseOnBlock places a comparator on a replaceable block.
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

func (c Comparator) outputPower(pos cube.Pos, tx *world.Tx) int {
	rearFace := c.Facing.Opposite().Face()
	rearPower := tx.RedstoneStoredPowerFrom(pos, rearFace)
	rearPos := pos.Side(rearFace)
	if b, ok := tx.BlockLoaded(rearPos); ok {
		if readable, ok := b.(world.RedstoneComparatorReadable); ok {
			rearPower = max(rearPower, redstonePower(readable.RedstoneComparatorOutput(rearPos, tx, c.Facing.Face())))
		}
	}

	sidePower := max(tx.RedstoneStoredPowerFrom(pos, c.Facing.RotateLeft().Face()), tx.RedstoneStoredPowerFrom(pos, c.Facing.RotateRight().Face()))
	if c.Subtract {
		return redstonePower(rearPower - sidePower)
	}
	if rearPower >= sidePower {
		return redstonePower(rearPower)
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

func allComparators() (comparators []world.Block) {
	for _, facing := range cube.Directions() {
		for _, subtract := range []bool{false, true} {
			comparators = append(comparators, Comparator{Facing: facing, Subtract: subtract}, Comparator{Facing: facing, Subtract: subtract, Powered: true})
		}
	}
	return
}

// Observer emits a short pulse when the block in front of it changes.
type Observer struct {
	solid
	sourceWaterDisplacer

	// Facing is the face observed by the observer.
	Facing cube.Face
	// Powered is true while the observer is emitting its pulse.
	Powered bool
}

// UseOnBlock places an observer with its output side facing the user.
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

func allObservers() (observers []world.Block) {
	for _, facing := range cube.Faces() {
		observers = append(observers, Observer{Facing: facing}, Observer{Facing: facing, Powered: true})
	}
	return
}

func redstonePlacementFace(user item.User) cube.Face {
	pitch := user.Rotation().Pitch()
	switch {
	case pitch > 45:
		return cube.FaceUp
	case pitch < -45:
		return cube.FaceDown
	default:
		return user.Rotation().Direction().Opposite().Face()
	}
}
