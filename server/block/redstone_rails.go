package block

import (
	"math/rand/v2"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

const detectorRailReleaseTicks = 2

// PoweredRail is a rail that accelerates or brakes carts depending on its powered state.
type PoweredRail struct {
	empty
	transparent
	sourceWaterDisplacer

	// Direction is the Bedrock rail direction state.
	Direction int
	// Powered is true when the rail is receiving redstone power.
	Powered bool
}

// UseOnBlock places a powered rail.
func (r PoweredRail) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	pos, _, used := firstReplaceable(tx, pos, face, r)
	if !used || !railSupported(tx, pos) {
		return false
	}
	if user != nil {
		r.Direction = railDirectionFromUser(user)
	}
	place(tx, pos, r, user, ctx)
	return placed(ctx)
}

// RedstonePowerUpdate updates the rail powered state.
func (r PoweredRail) RedstonePowerUpdate(_ cube.Pos, _ *world.Tx, power int) (world.Block, bool) {
	powered := power > 0
	if r.Powered == powered {
		return r, false
	}
	r.Powered = powered
	return r, true
}

// NeighbourUpdateTick breaks the rail if its supporting block is removed.
func (r PoweredRail) NeighbourUpdateTick(pos, _ cube.Pos, tx *world.Tx) {
	if !railSupported(tx, pos) {
		breakBlock(r, pos, tx)
	}
}

// BreakInfo ...
func (r PoweredRail) BreakInfo() BreakInfo {
	return newBreakInfo(0.7, alwaysHarvestable, pickaxeEffective, oneOf(r))
}

// Model ...
func (PoweredRail) Model() world.BlockModel {
	return railModel{}
}

// SideClosed ...
func (PoweredRail) SideClosed(cube.Pos, cube.Pos, *world.Tx) bool {
	return false
}

// EncodeItem ...
func (PoweredRail) EncodeItem() (name string, meta int16) {
	return "minecraft:golden_rail", 0
}

// EncodeBlock ...
func (r PoweredRail) EncodeBlock() (name string, properties map[string]any) {
	return "minecraft:golden_rail", map[string]any{"rail_direction": int32(railDirection(r.Direction)), "rail_data_bit": boolByte(r.Powered)}
}

func allPoweredRails() (rails []world.Block) {
	for direction := 0; direction < 6; direction++ {
		rails = append(rails, PoweredRail{Direction: direction}, PoweredRail{Direction: direction, Powered: true})
	}
	return
}

// DetectorRail is a rail that emits redstone while occupied.
type DetectorRail struct {
	empty
	transparent
	sourceWaterDisplacer

	// Direction is the Bedrock rail direction state.
	Direction int
	// Powered is true while an entity is detected on the rail.
	Powered bool
}

// UseOnBlock places a detector rail.
func (r DetectorRail) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	pos, _, used := firstReplaceable(tx, pos, face, r)
	if !used || !railSupported(tx, pos) {
		return false
	}
	if user != nil {
		r.Direction = railDirectionFromUser(user)
	}
	place(tx, pos, r, user, ctx)
	return placed(ctx)
}

// EntityInside powers the detector rail while an entity intersects it.
func (r DetectorRail) EntityInside(pos cube.Pos, tx *world.Tx, _ world.Entity) {
	if !r.Powered {
		r.Powered = true
		tx.SetBlock(pos, r, nil)
	}
	tx.ScheduleBlockUpdate(pos, r, redstoneTicks(detectorRailReleaseTicks))
}

// ScheduledTick releases a detector rail if no entity refreshed it.
func (r DetectorRail) ScheduledTick(pos cube.Pos, tx *world.Tx, _ *rand.Rand) {
	if !r.Powered {
		return
	}
	r.Powered = false
	tx.SetBlock(pos, r, nil)
}

// RedstonePower returns maximum power while occupied.
func (r DetectorRail) RedstonePower(cube.Pos, *world.Tx, cube.Face) int {
	if r.Powered {
		return 15
	}
	return 0
}

// RedstoneComparatorOutput returns a simple occupied signal for detector rails.
func (r DetectorRail) RedstoneComparatorOutput(cube.Pos, *world.Tx, cube.Face) int {
	if r.Powered {
		return 15
	}
	return 0
}

// NeighbourUpdateTick breaks the rail if its supporting block is removed.
func (r DetectorRail) NeighbourUpdateTick(pos, _ cube.Pos, tx *world.Tx) {
	if !railSupported(tx, pos) {
		breakBlock(r, pos, tx)
	}
}

// BreakInfo ...
func (r DetectorRail) BreakInfo() BreakInfo {
	return newBreakInfo(0.7, alwaysHarvestable, pickaxeEffective, oneOf(r))
}

// Model ...
func (DetectorRail) Model() world.BlockModel {
	return railModel{}
}

// SideClosed ...
func (DetectorRail) SideClosed(cube.Pos, cube.Pos, *world.Tx) bool {
	return false
}

// EncodeItem ...
func (DetectorRail) EncodeItem() (name string, meta int16) {
	return "minecraft:detector_rail", 0
}

// EncodeBlock ...
func (r DetectorRail) EncodeBlock() (name string, properties map[string]any) {
	return "minecraft:detector_rail", map[string]any{"rail_direction": int32(railDirection(r.Direction)), "rail_data_bit": boolByte(r.Powered)}
}

func allDetectorRails() (rails []world.Block) {
	for direction := 0; direction < 6; direction++ {
		rails = append(rails, DetectorRail{Direction: direction}, DetectorRail{Direction: direction, Powered: true})
	}
	return
}

// ActivatorRail is a rail that reacts to redstone power.
type ActivatorRail struct {
	empty
	transparent
	sourceWaterDisplacer

	// Direction is the Bedrock rail direction state.
	Direction int
	// Powered is true when the rail is receiving redstone power.
	Powered bool
}

// UseOnBlock places an activator rail.
func (r ActivatorRail) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	pos, _, used := firstReplaceable(tx, pos, face, r)
	if !used || !railSupported(tx, pos) {
		return false
	}
	if user != nil {
		r.Direction = railDirectionFromUser(user)
	}
	place(tx, pos, r, user, ctx)
	return placed(ctx)
}

// RedstonePowerUpdate updates the rail powered state.
func (r ActivatorRail) RedstonePowerUpdate(_ cube.Pos, _ *world.Tx, power int) (world.Block, bool) {
	powered := power > 0
	if r.Powered == powered {
		return r, false
	}
	r.Powered = powered
	return r, true
}

// NeighbourUpdateTick breaks the rail if its supporting block is removed.
func (r ActivatorRail) NeighbourUpdateTick(pos, _ cube.Pos, tx *world.Tx) {
	if !railSupported(tx, pos) {
		breakBlock(r, pos, tx)
	}
}

// BreakInfo ...
func (r ActivatorRail) BreakInfo() BreakInfo {
	return newBreakInfo(0.7, alwaysHarvestable, pickaxeEffective, oneOf(r))
}

// Model ...
func (ActivatorRail) Model() world.BlockModel {
	return railModel{}
}

// SideClosed ...
func (ActivatorRail) SideClosed(cube.Pos, cube.Pos, *world.Tx) bool {
	return false
}

// EncodeItem ...
func (ActivatorRail) EncodeItem() (name string, meta int16) {
	return "minecraft:activator_rail", 0
}

// EncodeBlock ...
func (r ActivatorRail) EncodeBlock() (name string, properties map[string]any) {
	return "minecraft:activator_rail", map[string]any{"rail_direction": int32(railDirection(r.Direction)), "rail_data_bit": boolByte(r.Powered)}
}

func allActivatorRails() (rails []world.Block) {
	for direction := 0; direction < 6; direction++ {
		rails = append(rails, ActivatorRail{Direction: direction}, ActivatorRail{Direction: direction, Powered: true})
	}
	return
}

func railDirection(direction int) int {
	return max(0, min(direction, 5))
}

func railDirectionFromUser(user item.User) int {
	switch user.Rotation().Direction() {
	case cube.East, cube.West:
		return 1
	default:
		return 0
	}
}

func railSupported(tx *world.Tx, pos cube.Pos) bool {
	below := pos.Side(cube.FaceDown)
	return tx.Block(below).Model().FaceSolid(below, cube.FaceUp, tx)
}

type railModel struct{}

func (railModel) BBox(cube.Pos, world.BlockSource) []cube.BBox {
	return []cube.BBox{cube.Box(0, 0, 0, 1, 0.125, 1)}
}

func (railModel) FaceSolid(cube.Pos, cube.Face, world.BlockSource) bool {
	return false
}
