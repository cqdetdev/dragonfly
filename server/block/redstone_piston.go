package block

import (
	"math/rand/v2"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/sound"
	"github.com/go-gl/mathgl/mgl64"
)

const pistonPushLimit = 12

const (
	pistonStateRetracted uint8 = iota
	pistonStateExtending
	pistonStateExtended
	pistonStateRetracting
)

var (
	_ world.NBTer                 = Piston{}
	_ world.RedstonePowerConsumer = Piston{}
	_ world.ScheduledTicker       = Piston{}
	_ world.NBTer                 = StickyPiston{}
	_ world.RedstonePowerConsumer = StickyPiston{}
	_ world.ScheduledTicker       = StickyPiston{}
)

// Piston is a redstone-powered block mover. It stores its extension state in NBT because the embedded Bedrock
// piston state table only exposes facing_direction.
type Piston struct {
	solid

	// Facing is the direction the piston pushes towards.
	Facing cube.Face
	// Extended is true after the piston has successfully extended.
	Extended bool
}

// UseOnBlock places a piston facing the user.
func (p Piston) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	pos, _, used := firstReplaceable(tx, pos, face, p)
	if !used {
		return false
	}
	if user != nil {
		p.Facing = calculateFace(user, pos)
	}
	place(tx, pos, p, user, ctx)
	return placed(ctx)
}

// RedstonePowerUpdate schedules an extension or retraction when the input power no longer matches the piston state.
func (p Piston) RedstonePowerUpdate(pos cube.Pos, tx *world.Tx, power int) (world.Block, bool) {
	if tx == nil {
		return p, false
	}
	if power > 0 && !p.Extended {
		tx.ScheduleBlockUpdate(pos, p, redstoneTicks(1))
	} else if power == 0 && p.Extended {
		tx.ScheduleBlockUpdate(pos, p, redstoneTicks(1))
	}
	return p, false
}

// NeighbourUpdateTick keeps the stored extended state in sync if the piston arm is removed.
func (p Piston) NeighbourUpdateTick(pos, _ cube.Pos, tx *world.Tx) {
	if !p.Extended {
		return
	}
	if _, ok := tx.Block(pos.Side(p.Facing)).(PistonArm); !ok {
		p.Extended = false
		setPistonState(pos, p, tx)
	}
}

// ScheduledTick executes a pending extension or retraction.
func (p Piston) ScheduledTick(pos cube.Pos, tx *world.Tx, _ *rand.Rand) {
	if tx.RedstonePower(pos) > 0 {
		if p.Extended || !pistonPush(pos, p.Facing, tx) {
			return
		}
		p.Extended = true
		setPistonState(pos, p, tx)
		setPistonArm(pos, p.Facing, false, tx)
		tx.PlaySound(pos.Vec3Centre(), sound.PistonOut{})
		return
	}
	if !p.Extended {
		return
	}
	removePistonArm(pos, p.Facing, tx)
	p.Extended = false
	setPistonState(pos, p, tx)
	tx.PlaySound(pos.Vec3Centre(), sound.PistonIn{})
}

// DecodeNBT decodes the piston's extension state.
func (p Piston) DecodeNBT(data map[string]any) any {
	p.Extended = pistonNBTBool(data["extended"]) || nbtInt(data["State"]) == int(pistonStateExtended)
	return p
}

// EncodeNBT encodes the piston's extension state.
func (p Piston) EncodeNBT() map[string]any {
	return pistonArmNBT(p.Facing, p.Extended, false)
}

// BreakInfo ...
func (p Piston) BreakInfo() BreakInfo {
	return newBreakInfo(1.5, alwaysHarvestable, pickaxeEffective, oneOf(p)).withBlastResistance(2.5)
}

// EncodeItem ...
func (Piston) EncodeItem() (name string, meta int16) {
	return "minecraft:piston", 0
}

// EncodeBlock ...
func (p Piston) EncodeBlock() (string, map[string]any) {
	return "minecraft:piston", map[string]any{"facing_direction": pistonClientFacingDirection(p.Facing)}
}

// StickyPiston is a piston that pulls one block back when retracting.
type StickyPiston struct {
	solid

	// Facing is the direction the sticky piston pushes towards.
	Facing cube.Face
	// Extended is true after the sticky piston has successfully extended.
	Extended bool
}

// UseOnBlock places a sticky piston facing the user.
func (p StickyPiston) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	pos, _, used := firstReplaceable(tx, pos, face, p)
	if !used {
		return false
	}
	if user != nil {
		p.Facing = calculateFace(user, pos)
	}
	place(tx, pos, p, user, ctx)
	return placed(ctx)
}

// RedstonePowerUpdate schedules an extension or retraction when the input power no longer matches the piston state.
func (p StickyPiston) RedstonePowerUpdate(pos cube.Pos, tx *world.Tx, power int) (world.Block, bool) {
	if tx == nil {
		return p, false
	}
	if power > 0 && !p.Extended {
		tx.ScheduleBlockUpdate(pos, p, redstoneTicks(1))
	} else if power == 0 && p.Extended {
		tx.ScheduleBlockUpdate(pos, p, redstoneTicks(1))
	}
	return p, false
}

// NeighbourUpdateTick keeps the stored extended state in sync if the piston arm is removed.
func (p StickyPiston) NeighbourUpdateTick(pos, _ cube.Pos, tx *world.Tx) {
	if !p.Extended {
		return
	}
	if _, ok := tx.Block(pos.Side(p.Facing)).(PistonArm); !ok {
		p.Extended = false
		setPistonState(pos, p, tx)
	}
}

// ScheduledTick executes a pending extension or sticky retraction.
func (p StickyPiston) ScheduledTick(pos cube.Pos, tx *world.Tx, _ *rand.Rand) {
	if tx.RedstonePower(pos) > 0 {
		if p.Extended || !pistonPush(pos, p.Facing, tx) {
			return
		}
		p.Extended = true
		setPistonState(pos, p, tx)
		setPistonArm(pos, p.Facing, true, tx)
		tx.PlaySound(pos.Vec3Centre(), sound.PistonOut{})
		return
	}
	if !p.Extended {
		return
	}
	removePistonArm(pos, p.Facing, tx)
	_ = pistonPull(pos, p.Facing, tx)
	p.Extended = false
	setPistonState(pos, p, tx)
	tx.PlaySound(pos.Vec3Centre(), sound.PistonIn{})
}

// PistonArm is the visible piston head shown while a piston is extended.
type PistonArm struct {
	solid

	// Facing is the direction the arm extends towards.
	Facing cube.Face
	// Sticky is true if the arm belongs to a sticky piston.
	Sticky bool
}

// BreakInfo ...
func (p PistonArm) BreakInfo() BreakInfo {
	return newBreakInfo(0.5, alwaysHarvestable, nothingEffective, oneOf())
}

// EncodeBlock ...
func (p PistonArm) EncodeBlock() (string, map[string]any) {
	if p.Sticky {
		return "minecraft:sticky_piston_arm_collision", map[string]any{"facing_direction": pistonClientFacingDirection(p.Facing)}
	}
	return "minecraft:piston_arm_collision", map[string]any{"facing_direction": pistonClientFacingDirection(p.Facing)}
}

// NeighbourUpdateTick removes orphaned piston arms.
func (p PistonArm) NeighbourUpdateTick(pos, _ cube.Pos, tx *world.Tx) {
	basePos := pos.Side(p.Facing.Opposite())
	if p.Sticky {
		if base, ok := tx.Block(basePos).(StickyPiston); ok && base.Extended && base.Facing == p.Facing {
			return
		}
	} else if base, ok := tx.Block(basePos).(Piston); ok && base.Extended && base.Facing == p.Facing {
		return
	}
	tx.SetBlock(pos, nil, &world.SetOpts{DisableRedstoneUpdates: true})
}

// DecodeNBT decodes the sticky piston's extension state.
func (p StickyPiston) DecodeNBT(data map[string]any) any {
	p.Extended = pistonNBTBool(data["extended"]) || nbtInt(data["State"]) == int(pistonStateExtended)
	return p
}

// EncodeNBT encodes the sticky piston's extension state.
func (p StickyPiston) EncodeNBT() map[string]any {
	return pistonArmNBT(p.Facing, p.Extended, true)
}

// BreakInfo ...
func (p StickyPiston) BreakInfo() BreakInfo {
	return newBreakInfo(1.5, alwaysHarvestable, pickaxeEffective, oneOf(p)).withBlastResistance(2.5)
}

// EncodeItem ...
func (StickyPiston) EncodeItem() (name string, meta int16) {
	return "minecraft:sticky_piston", 0
}

// EncodeBlock ...
func (p StickyPiston) EncodeBlock() (string, map[string]any) {
	return "minecraft:sticky_piston", map[string]any{"facing_direction": pistonClientFacingDirection(p.Facing)}
}

func pistonArmNBT(facing cube.Face, extended, sticky bool) map[string]any {
	state := pistonStateRetracted
	progress := float32(0)
	if extended {
		state = pistonStateExtended
		progress = 1
	}
	return map[string]any{
		"id":             "PistonArm",
		"Progress":       progress,
		"LastProgress":   progress,
		"isMovable":      uint8(1),
		"facing":         pistonClientFacingDirection(facing),
		"Extending":      boolByte(extended),
		"powered":        boolByte(extended),
		"AttachedBlocks": []int32{},
		"BreakBlocks":    []int32{},
		"Sticky":         boolByte(sticky),
		"State":          state,
		"NewState":       state,
		"extended":       boolByte(extended),
	}
}

func pistonClientFacingDirection(face cube.Face) int32 {
	if face == cube.FaceUp || face == cube.FaceDown {
		return pistonFacingDirection(face)
	}
	return pistonFacingDirection(face.Opposite())
}

func pistonFacingDirection(face cube.Face) int32 {
	switch face {
	case cube.FaceDown:
		return 0
	case cube.FaceUp:
		return 1
	case cube.FaceNorth:
		return 2
	case cube.FaceSouth:
		return 3
	case cube.FaceWest:
		return 4
	case cube.FaceEast:
		return 5
	default:
		return int32(face)
	}
}

type pistonMove struct {
	from, to cube.Pos
	block    world.Block
}

type pistonBlockLookup func(cube.Pos) (world.Block, bool)

func pistonPush(pos cube.Pos, face cube.Face, tx *world.Tx) bool {
	moves, ok := pistonPushPlan(pos, face, tx.Range(), tx.BlockLoaded)
	if !ok {
		return false
	}
	applyPistonMoves(moves, tx)
	return true
}

func pistonPull(pos cube.Pos, face cube.Face, tx *world.Tx) bool {
	move, ok := pistonPullPlan(pos, face, tx.Range(), tx.BlockLoaded)
	if !ok {
		return false
	}
	if move.block == nil {
		return true
	}
	applyPistonMoves([]pistonMove{move}, tx)
	return true
}

func pistonPushPlan(pos cube.Pos, face cube.Face, r cube.Range, blockAt pistonBlockLookup) ([]pistonMove, bool) {
	blocks := make([]pistonMove, 0, pistonPushLimit)
	for cursor := pos.Side(face); ; cursor = cursor.Side(face) {
		if cursor.OutOfBounds(r) {
			return nil, false
		}
		b, ok := blockAt(cursor)
		if !ok {
			return nil, false
		}
		if _, air := b.(Air); air {
			moves := make([]pistonMove, 0, len(blocks))
			for i := len(blocks) - 1; i >= 0; i-- {
				move := blocks[i]
				move.to = move.from.Side(face)
				moves = append(moves, move)
			}
			return moves, true
		}
		if len(blocks) == pistonPushLimit || !pistonMovableBlock(b) {
			return nil, false
		}
		blocks = append(blocks, pistonMove{from: cursor, block: b})
	}
}

func pistonPullPlan(pos cube.Pos, face cube.Face, r cube.Range, blockAt pistonBlockLookup) (pistonMove, bool) {
	to := pos.Side(face)
	from := to.Side(face)
	if to.OutOfBounds(r) || from.OutOfBounds(r) {
		return pistonMove{}, false
	}
	b, ok := blockAt(to)
	if !ok {
		return pistonMove{}, false
	}
	if _, air := b.(Air); !air {
		return pistonMove{}, false
	}
	b, ok = blockAt(from)
	if !ok {
		return pistonMove{}, false
	}
	if _, air := b.(Air); air {
		return pistonMove{}, true
	}
	if !pistonMovableBlock(b) {
		return pistonMove{}, false
	}
	return pistonMove{from: from, to: to, block: b}, true
}

func applyPistonMoves(moves []pistonMove, tx *world.Tx) {
	if len(moves) == 0 {
		return
	}
	for _, move := range moves {
		tx.SetBlock(move.to, move.block, nil)
	}
	tx.SetBlock(moves[len(moves)-1].from, nil, nil)
}

func pistonMovableBlock(b world.Block) bool {
	switch b.(type) {
	case Air, Bedrock, Barrier, InvisibleBedrock, Obsidian, ReinforcedDeepslate, PistonArm,
		Lever, Button, PressurePlate, RedstoneWire, RedstoneTorch, Repeater, Comparator, TripwireHook, String:
		return false
	}
	if _, ok := b.(world.Liquid); ok {
		return false
	}
	if _, ok := b.(world.NBTer); ok {
		return false
	}
	if _, ok := b.(Replaceable); ok {
		return false
	}
	return true
}

func setPistonState(pos cube.Pos, b world.Block, tx *world.Tx) {
	tx.SetBlock(pos, b, &world.SetOpts{DisableBlockUpdates: true, DisableRedstoneUpdates: true})
}

func setPistonArm(pos cube.Pos, face cube.Face, sticky bool, tx *world.Tx) {
	tx.SetBlock(pos.Side(face), PistonArm{Facing: face, Sticky: sticky}, &world.SetOpts{DisableRedstoneUpdates: true})
}

func removePistonArm(pos cube.Pos, face cube.Face, tx *world.Tx) {
	armPos := pos.Side(face)
	if _, ok := tx.Block(armPos).(PistonArm); ok {
		tx.SetBlock(armPos, nil, &world.SetOpts{DisableRedstoneUpdates: true})
	}
}

func allPistons() (pistons []world.Block) {
	for _, facing := range cube.Faces() {
		pistons = append(pistons, Piston{Facing: facing})
	}
	return pistons
}

func allPistonArms() (arms []world.Block) {
	for _, facing := range cube.Faces() {
		arms = append(arms, PistonArm{Facing: facing}, PistonArm{Facing: facing, Sticky: true})
	}
	return arms
}

func allStickyPistons() (pistons []world.Block) {
	for _, facing := range cube.Faces() {
		pistons = append(pistons, StickyPiston{Facing: facing})
	}
	return pistons
}

func redstonePistonStatesSupported(name string) bool {
	for _, facing := range cube.Faces() {
		if _, ok := world.BlockByName(name, map[string]any{"facing_direction": int32(facing)}); !ok {
			return false
		}
	}
	return true
}

func pistonNBTBool(v any) bool {
	switch v := v.(type) {
	case bool:
		return v
	case int:
		return v != 0
	case int8:
		return v != 0
	case int16:
		return v != 0
	case int32:
		return v != 0
	case int64:
		return v != 0
	case uint8:
		return v != 0
	case uint16:
		return v != 0
	case uint32:
		return v != 0
	case uint64:
		return v != 0
	default:
		return false
	}
}
