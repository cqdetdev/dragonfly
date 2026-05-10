package block

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/model"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/sound"
	"github.com/go-gl/mathgl/mgl64"
	"time"
)

// WoodDoor is a block that can be used as an openable 1x2 barrier.
type WoodDoor struct {
	transparent
	bass
	sourceWaterDisplacer

	// Wood is the type of wood of the door. This field must have one of the values found in the material
	// package.
	Wood WoodType
	// Facing is the direction that the door opens towards. When closed, the door sits on the side of its
	// block on the opposite direction.
	Facing cube.Direction
	// Open is whether the door is open.
	Open bool
	// Top is whether the block is the top or bottom half of a door.
	Top bool
	// Right is whether the door hinge is on the right side.
	Right bool
}

// FlammabilityInfo ...
func (d WoodDoor) FlammabilityInfo() FlammabilityInfo {
	if !d.Wood.Flammable() {
		return newFlammabilityInfo(0, 0, false)
	}
	return newFlammabilityInfo(0, 0, true)
}

// FuelInfo ...
func (d WoodDoor) FuelInfo() item.FuelInfo {
	if !d.Wood.Flammable() {
		return item.FuelInfo{}
	}
	return newFuelInfo(time.Second * 10)
}

// Model ...
func (d WoodDoor) Model() world.BlockModel {
	return model.Door{Facing: d.Facing, Open: d.Open, Right: d.Right}
}

// NeighbourUpdateTick ...
func (d WoodDoor) NeighbourUpdateTick(pos, _ cube.Pos, tx *world.Tx) {
	if d.Top {
		if _, ok := tx.Block(pos.Side(cube.FaceDown)).(WoodDoor); !ok {
			breakBlockNoDrops(d, pos, tx)
		}
	} else if solid := tx.Block(pos.Side(cube.FaceDown)).Model().FaceSolid(pos.Side(cube.FaceDown), cube.FaceUp, tx); !solid {
		breakBlock(d, pos, tx)
	} else if _, ok := tx.Block(pos.Side(cube.FaceUp)).(WoodDoor); !ok {
		breakBlockNoDrops(d, pos, tx)
	}
}

// UseOnBlock handles the directional placing of doors
func (d WoodDoor) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	if face != cube.FaceUp {
		// Doors can only be placed when clicking the top face.
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
	// The side the door hinge is on can be affected by the blocks to the left and right of the door. In particular,
	// opaque blocks on the right side of the door with transparent blocks on the left side result in a right sided
	// door hinge.
	if diffuser, ok := right.(LightDiffuser); !ok || diffuser.LightDiffusionLevel() != 0 {
		if diffuser, ok := left.(LightDiffuser); ok && diffuser.LightDiffusionLevel() == 0 {
			d.Right = true
		}
	}

	ctx.IgnoreBBox = true
	place(tx, pos, d, user, ctx)
	place(tx, pos.Side(cube.FaceUp), WoodDoor{Wood: d.Wood, Facing: d.Facing, Top: true, Right: d.Right}, user, ctx)
	ctx.CountSub = 1
	return placed(ctx)
}

// Activate ...
func (d WoodDoor) Activate(pos cube.Pos, _ cube.Face, tx *world.Tx, _ item.User, _ *item.UseContext) bool {
	d.Open = !d.Open
	tx.SetBlock(pos, d, nil)

	otherPos := pos.Side(cube.Face(boolByte(!d.Top)))
	other := tx.Block(otherPos)
	if door, ok := other.(WoodDoor); ok {
		door.Open = d.Open
		tx.SetBlock(otherPos, door, nil)
	}
	if d.Open {
		tx.PlaySound(pos.Vec3Centre(), sound.DoorOpen{Block: d})
		return true
	}
	tx.PlaySound(pos.Vec3Centre(), sound.DoorClose{Block: d})
	return true
}

// RedstonePowerUpdate returns a door half with its open state matching the redstone power supplied.
func (d WoodDoor) RedstonePowerUpdate(pos cube.Pos, tx *world.Tx, power int) (world.Block, bool) {
	open := d.redstoneDoorPower(pos, tx, power) > 0
	if d.Open == open {
		return d, false
	}
	d.Open = open
	return d, true
}

// RedstonePowerTransitionUpdate opens on a rising redstone edge and closes only after a previous powered state.
func (d WoodDoor) RedstonePowerTransitionUpdate(pos cube.Pos, tx *world.Tx, oldPower, newPower int) (world.Block, bool) {
	open, changed := redstoneOpenableTransition(d.Open, oldPower, d.redstoneDoorPower(pos, tx, newPower))
	if !changed {
		return d, false
	}
	d.Open = open
	return d, true
}

// RedstonePowerPostUpdate syncs the other door half after an uncancelled redstone update.
func (d WoodDoor) RedstonePowerPostUpdate(pos cube.Pos, tx *world.Tx, _, after world.Block, _, _ int) {
	door := after.(WoodDoor)
	otherPos := pos.Side(cube.Face(boolByte(!door.Top)))
	if other, ok := tx.Block(otherPos).(WoodDoor); ok && other.Open != door.Open {
		other.Open = door.Open
		tx.SetBlock(otherPos, other, &world.SetOpts{DisableBlockUpdates: true, DisableRedstoneUpdates: true})
	}
}

// RedstonePowerUpdateSound returns the sound for a redstone-driven door state change.
func (d WoodDoor) RedstonePowerUpdateSound(_ cube.Pos, _ *world.Tx, _ world.Block, after world.Block, _, _ int) world.Sound {
	door := after.(WoodDoor)
	if door.Open {
		return sound.DoorOpen{Block: door}
	}
	return sound.DoorClose{Block: door}
}

func (d WoodDoor) redstoneDoorPower(pos cube.Pos, tx *world.Tx, power int) int {
	if tx == nil {
		return power
	}
	otherPos := pos.Side(cube.Face(boolByte(!d.Top)))
	if _, ok := tx.Block(otherPos).(WoodDoor); ok {
		power = max(power, tx.RedstonePower(otherPos))
	}
	return power
}

// BreakInfo ...
func (d WoodDoor) BreakInfo() BreakInfo {
	return newBreakInfo(3, alwaysHarvestable, axeEffective, oneOf(d))
}

// SideClosed ...
func (d WoodDoor) SideClosed(cube.Pos, cube.Pos, *world.Tx) bool {
	return false
}

// EncodeItem ...
func (d WoodDoor) EncodeItem() (name string, meta int16) {
	if d.Wood == OakWood() {
		return "minecraft:wooden_door", 0
	}
	return "minecraft:" + d.Wood.String() + "_door", 0
}

// EncodeBlock ...
func (d WoodDoor) EncodeBlock() (name string, properties map[string]any) {
	if d.Wood == OakWood() {
		return "minecraft:wooden_door", map[string]any{"minecraft:cardinal_direction": d.Facing.RotateRight().String(), "door_hinge_bit": d.Right, "open_bit": d.Open, "upper_block_bit": d.Top}
	}
	return "minecraft:" + d.Wood.String() + "_door", map[string]any{"minecraft:cardinal_direction": d.Facing.RotateRight().String(), "door_hinge_bit": d.Right, "open_bit": d.Open, "upper_block_bit": d.Top}
}

// allDoors returns a list of all door types
func allDoors() (doors []world.Block) {
	for _, w := range WoodTypes() {
		for i := cube.Direction(0); i <= 3; i++ {
			doors = append(doors, WoodDoor{Wood: w, Facing: i, Open: false, Top: false, Right: false})
			doors = append(doors, WoodDoor{Wood: w, Facing: i, Open: false, Top: true, Right: false})
			doors = append(doors, WoodDoor{Wood: w, Facing: i, Open: true, Top: true, Right: false})
			doors = append(doors, WoodDoor{Wood: w, Facing: i, Open: true, Top: false, Right: false})
			doors = append(doors, WoodDoor{Wood: w, Facing: i, Open: false, Top: false, Right: true})
			doors = append(doors, WoodDoor{Wood: w, Facing: i, Open: false, Top: true, Right: true})
			doors = append(doors, WoodDoor{Wood: w, Facing: i, Open: true, Top: true, Right: true})
			doors = append(doors, WoodDoor{Wood: w, Facing: i, Open: true, Top: false, Right: true})
		}
	}
	return
}
