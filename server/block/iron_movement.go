package block

import (
	"math"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/model"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/sound"
	"github.com/go-gl/mathgl/mgl64"
)

// IronDoor is a redstone-only 1x2 barrier.
type IronDoor struct {
	transparent
	sourceWaterDisplacer

	// Facing is the direction that the door opens towards.
	Facing cube.Direction
	// Open is whether the door is open.
	Open bool
	// Top is whether the block is the top or bottom half of a door.
	Top bool
	// Right is whether the door hinge is on the right side.
	Right bool
}

// Model ...
func (d IronDoor) Model() world.BlockModel {
	return model.Door{Facing: d.Facing, Open: d.Open, Right: d.Right}
}

// NeighbourUpdateTick ...
func (d IronDoor) NeighbourUpdateTick(pos, _ cube.Pos, tx *world.Tx) {
	if d.Top {
		if _, ok := tx.Block(pos.Side(cube.FaceDown)).(IronDoor); !ok {
			breakBlockNoDrops(d, pos, tx)
		}
	} else if solid := tx.Block(pos.Side(cube.FaceDown)).Model().FaceSolid(pos.Side(cube.FaceDown), cube.FaceUp, tx); !solid {
		breakBlock(d, pos, tx)
	} else if _, ok := tx.Block(pos.Side(cube.FaceUp)).(IronDoor); !ok {
		breakBlockNoDrops(d, pos, tx)
	}
}

// UseOnBlock handles the directional placing of iron doors.
func (d IronDoor) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	if face != cube.FaceUp {
		return false
	}
	below := pos
	pos = pos.Side(cube.FaceUp)
	if !replaceableWith(tx, pos, d) || !replaceableWith(tx, pos.Side(cube.FaceUp), d) {
		return false
	}
	if !tx.Block(below).Model().FaceSolid(below, cube.FaceUp, tx) {
		return false
	}
	d.Facing = user.Rotation().Direction()
	left := tx.Block(pos.Side(d.Facing.RotateLeft().Face()))
	right := tx.Block(pos.Side(d.Facing.RotateRight().Face()))
	if _, ok := left.Model().(model.Door); ok {
		d.Right = true
	}
	if diffuser, ok := right.(LightDiffuser); !ok || diffuser.LightDiffusionLevel() != 0 {
		if diffuser, ok := left.(LightDiffuser); ok && diffuser.LightDiffusionLevel() == 0 {
			d.Right = true
		}
	}

	ctx.IgnoreBBox = true
	place(tx, pos, d, user, ctx)
	place(tx, pos.Side(cube.FaceUp), IronDoor{Facing: d.Facing, Top: true, Right: d.Right}, user, ctx)
	ctx.CountSub = 1
	return placed(ctx)
}

// Activate returns false because iron doors are redstone-only.
func (IronDoor) Activate(cube.Pos, cube.Face, *world.Tx, item.User, *item.UseContext) bool {
	return false
}

// RedstonePowerUpdate returns a door half with its open state matching the redstone power supplied.
func (d IronDoor) RedstonePowerUpdate(pos cube.Pos, tx *world.Tx, power int) (world.Block, bool) {
	open := d.redstoneDoorPower(pos, tx, power) > 0
	if d.Open == open {
		return d, false
	}
	d.Open = open
	return d, true
}

// RedstonePowerTransitionUpdate opens on a rising redstone edge and closes only after a previous powered state.
func (d IronDoor) RedstonePowerTransitionUpdate(pos cube.Pos, tx *world.Tx, oldPower, newPower int) (world.Block, bool) {
	open, changed := redstoneOpenableTransition(d.Open, oldPower, d.redstoneDoorPower(pos, tx, newPower))
	if !changed {
		return d, false
	}
	d.Open = open
	return d, true
}

// RedstonePowerPostUpdate syncs the other door half after an uncancelled redstone update.
func (d IronDoor) RedstonePowerPostUpdate(pos cube.Pos, tx *world.Tx, _, after world.Block, _, _ int) {
	door := after.(IronDoor)
	otherPos := pos.Side(cube.Face(boolByte(!door.Top)))
	if other, ok := tx.Block(otherPos).(IronDoor); ok && other.Open != door.Open {
		other.Open = door.Open
		tx.SetBlock(otherPos, other, &world.SetOpts{DisableBlockUpdates: true, DisableRedstoneUpdates: true})
	}
}

// RedstonePowerUpdateSound returns the sound for a redstone-driven iron door state change.
func (d IronDoor) RedstonePowerUpdateSound(_ cube.Pos, _ *world.Tx, _ world.Block, after world.Block, _, _ int) world.Sound {
	door := after.(IronDoor)
	if door.Open {
		return sound.DoorOpen{Block: door}
	}
	return sound.DoorClose{Block: door}
}

func (d IronDoor) redstoneDoorPower(pos cube.Pos, tx *world.Tx, power int) int {
	if tx == nil {
		return power
	}
	otherPos := pos.Side(cube.Face(boolByte(!d.Top)))
	if _, ok := tx.Block(otherPos).(IronDoor); ok {
		power = max(power, tx.RedstonePower(otherPos))
	}
	return power
}

// BreakInfo ...
func (d IronDoor) BreakInfo() BreakInfo {
	return newBreakInfo(5, pickaxeHarvestable, pickaxeEffective, oneOf(d)).withBlastResistance(25)
}

// SideClosed ...
func (IronDoor) SideClosed(cube.Pos, cube.Pos, *world.Tx) bool {
	return false
}

// EncodeItem ...
func (IronDoor) EncodeItem() (name string, meta int16) {
	return "minecraft:iron_door", 0
}

// EncodeBlock ...
func (d IronDoor) EncodeBlock() (name string, properties map[string]any) {
	return "minecraft:iron_door", map[string]any{
		"minecraft:cardinal_direction": d.Facing.RotateRight().String(),
		"door_hinge_bit":               d.Right,
		"open_bit":                     d.Open,
		"upper_block_bit":              d.Top,
	}
}

func allIronDoors() (doors []world.Block) {
	for i := cube.Direction(0); i <= 3; i++ {
		doors = append(doors, IronDoor{Facing: i, Open: false, Top: false, Right: false})
		doors = append(doors, IronDoor{Facing: i, Open: false, Top: true, Right: false})
		doors = append(doors, IronDoor{Facing: i, Open: true, Top: true, Right: false})
		doors = append(doors, IronDoor{Facing: i, Open: true, Top: false, Right: false})
		doors = append(doors, IronDoor{Facing: i, Open: false, Top: false, Right: true})
		doors = append(doors, IronDoor{Facing: i, Open: false, Top: true, Right: true})
		doors = append(doors, IronDoor{Facing: i, Open: true, Top: true, Right: true})
		doors = append(doors, IronDoor{Facing: i, Open: true, Top: false, Right: true})
	}
	return
}

// IronTrapdoor is a redstone-only 1x1 barrier.
type IronTrapdoor struct {
	transparent
	sourceWaterDisplacer

	// Facing is the direction the trapdoor is facing.
	Facing cube.Direction
	// Open is whether the trapdoor is open.
	Open bool
	// Top is whether the trapdoor occupies the top or bottom part of a block.
	Top bool
}

// Model ...
func (t IronTrapdoor) Model() world.BlockModel {
	return model.Trapdoor{Facing: t.Facing, Top: t.Top, Open: t.Open}
}

// UseOnBlock handles the directional placing of iron trapdoors.
func (t IronTrapdoor) UseOnBlock(pos cube.Pos, face cube.Face, clickPos mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	pos, face, used := firstReplaceable(tx, pos, face, t)
	if !used {
		return false
	}
	t.Facing = user.Rotation().Direction().Opposite()
	t.Top = (clickPos.Y() > 0.5 && face != cube.FaceUp) || face == cube.FaceDown
	place(tx, pos, t, user, ctx)
	return placed(ctx)
}

// Activate returns false because iron trapdoors are redstone-only.
func (IronTrapdoor) Activate(cube.Pos, cube.Face, *world.Tx, item.User, *item.UseContext) bool {
	return false
}

// RedstonePowerUpdate returns a trapdoor with its open state matching the redstone power supplied.
func (t IronTrapdoor) RedstonePowerUpdate(_ cube.Pos, _ *world.Tx, power int) (world.Block, bool) {
	open := power > 0
	if t.Open == open {
		return t, false
	}
	t.Open = open
	return t, true
}

// RedstonePowerTransitionUpdate opens on a rising redstone edge and closes only after a previous powered state.
func (t IronTrapdoor) RedstonePowerTransitionUpdate(_ cube.Pos, _ *world.Tx, oldPower, newPower int) (world.Block, bool) {
	open, changed := redstoneOpenableTransition(t.Open, oldPower, newPower)
	if !changed {
		return t, false
	}
	t.Open = open
	return t, true
}

// RedstonePowerUpdateSound returns the sound for a redstone-driven iron trapdoor state change.
func (t IronTrapdoor) RedstonePowerUpdateSound(_ cube.Pos, _ *world.Tx, _ world.Block, after world.Block, _, _ int) world.Sound {
	trapdoor := after.(IronTrapdoor)
	if trapdoor.Open {
		return sound.TrapdoorOpen{Block: trapdoor}
	}
	return sound.TrapdoorClose{Block: trapdoor}
}

// BreakInfo ...
func (t IronTrapdoor) BreakInfo() BreakInfo {
	return newBreakInfo(5, pickaxeHarvestable, pickaxeEffective, oneOf(t)).withBlastResistance(25)
}

// SideClosed ...
func (IronTrapdoor) SideClosed(cube.Pos, cube.Pos, *world.Tx) bool {
	return false
}

// EncodeItem ...
func (IronTrapdoor) EncodeItem() (name string, meta int16) {
	return "minecraft:iron_trapdoor", 0
}

// EncodeBlock ...
func (t IronTrapdoor) EncodeBlock() (name string, properties map[string]any) {
	return "minecraft:iron_trapdoor", map[string]any{
		"direction":       int32(math.Abs(float64(t.Facing) - 3)),
		"open_bit":        t.Open,
		"upside_down_bit": t.Top,
	}
}

func allIronTrapdoors() (trapdoors []world.Block) {
	for i := cube.Direction(0); i <= 3; i++ {
		trapdoors = append(trapdoors, IronTrapdoor{Facing: i, Open: false, Top: false})
		trapdoors = append(trapdoors, IronTrapdoor{Facing: i, Open: false, Top: true})
		trapdoors = append(trapdoors, IronTrapdoor{Facing: i, Open: true, Top: true})
		trapdoors = append(trapdoors, IronTrapdoor{Facing: i, Open: true, Top: false})
	}
	return
}
