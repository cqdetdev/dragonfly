package block

import (
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

func TestRepeaterRedstonePower(t *testing.T) {
	r := Repeater{Facing: cube.East, Powered: true}
	if power := r.RedstonePower(cube.Pos{}, nil, cube.FaceEast); power != 15 {
		t.Fatalf("powered repeater output = %d, want 15", power)
	}
	if power := r.RedstonePower(cube.Pos{}, nil, cube.FaceWest); power != 0 {
		t.Fatalf("powered repeater rear output = %d, want 0", power)
	}
	if power := (Repeater{Facing: cube.East}).RedstonePower(cube.Pos{}, nil, cube.FaceEast); power != 0 {
		t.Fatalf("unpowered repeater output = %d, want 0", power)
	}
}

func TestRepeaterEncodeBlockClampsDelay(t *testing.T) {
	name, props := (Repeater{Facing: cube.North, Delay: 8, Powered: true}).EncodeBlock()
	if name != "minecraft:powered_repeater" {
		t.Fatalf("repeater block name = %q, want powered repeater", name)
	}
	if delay := props["repeater_delay"]; delay != int32(3) {
		t.Fatalf("repeater_delay = %v, want 3", delay)
	}
	if direction := props["minecraft:cardinal_direction"]; direction != "south" {
		t.Fatalf("minecraft:cardinal_direction = %v, want south", direction)
	}
	world.DefaultBlockRegistry.Finalize()
	if _, ok := world.BlockByName(name, props); !ok {
		t.Fatalf("BlockByName(%s, %#v) was not found", name, props)
	}
}

func TestRepeaterHashIncludesDelay(t *testing.T) {
	_, delay0 := (Repeater{Facing: cube.North, Delay: 0}).Hash()
	_, delay1 := (Repeater{Facing: cube.North, Delay: 1}).Hash()
	if delay0 == delay1 {
		t.Fatalf("repeater hash should include Delay")
	}
}

func TestComparatorRedstonePower(t *testing.T) {
	c := Comparator{Facing: cube.South, Powered: true}
	if power := c.RedstonePower(cube.Pos{}, nil, cube.FaceSouth); power != 15 {
		t.Fatalf("powered comparator output = %d, want 15", power)
	}
	if power := c.RedstonePower(cube.Pos{}, nil, cube.FaceNorth); power != 0 {
		t.Fatalf("powered comparator rear output = %d, want 0", power)
	}
	if power := (Comparator{Facing: cube.South}).RedstonePower(cube.Pos{}, nil, cube.FaceSouth); power != 0 {
		t.Fatalf("unpowered comparator output = %d, want 0", power)
	}
}

func TestComparatorEncodeBlock(t *testing.T) {
	name, props := (Comparator{Facing: cube.West, Subtract: true, Powered: true}).EncodeBlock()
	if name != "minecraft:powered_comparator" {
		t.Fatalf("comparator block name = %q, want powered comparator", name)
	}
	if subtract := props["output_subtract_bit"]; subtract != uint8(1) {
		t.Fatalf("output_subtract_bit = %v, want 1", subtract)
	}
	if lit := props["output_lit_bit"]; lit != uint8(1) {
		t.Fatalf("output_lit_bit = %v, want 1", lit)
	}
	if direction := props["minecraft:cardinal_direction"]; direction != "east" {
		t.Fatalf("minecraft:cardinal_direction = %v, want east", direction)
	}
	world.DefaultBlockRegistry.Finalize()
	if _, ok := world.BlockByName(name, props); !ok {
		t.Fatalf("BlockByName(%s, %#v) was not found", name, props)
	}
}

func TestObserverRedstonePower(t *testing.T) {
	o := Observer{Facing: cube.FaceNorth, Powered: true}
	if power := o.RedstonePower(cube.Pos{}, nil, cube.FaceSouth); power != 15 {
		t.Fatalf("powered observer back output = %d, want 15", power)
	}
	if power := o.RedstonePower(cube.Pos{}, nil, cube.FaceNorth); power != 0 {
		t.Fatalf("powered observer front output = %d, want 0", power)
	}
	if power := (Observer{Facing: cube.FaceNorth}).RedstonePower(cube.Pos{}, nil, cube.FaceSouth); power != 0 {
		t.Fatalf("unpowered observer output = %d, want 0", power)
	}
}

func TestObserverEncodeBlock(t *testing.T) {
	name, props := (Observer{Facing: cube.FaceEast, Powered: true}).EncodeBlock()
	if facing := props["minecraft:facing_direction"]; facing != "east" {
		t.Fatalf("facing_direction = %v, want east", facing)
	}
	if powered := props["powered_bit"]; powered != uint8(1) {
		t.Fatalf("powered_bit = %v, want 1", powered)
	}
	if _, ok := world.BlockByName(name, props); !ok {
		t.Fatalf("BlockByName(%s, %#v) was not found", name, props)
	}
}

func TestRedstoneLogicBlockStateCounts(t *testing.T) {
	if count := len(allRepeaters()); count != len(cube.Directions())*4*2 {
		t.Fatalf("allRepeaters returned %d states", count)
	}
	if count := len(allComparators()); count != len(cube.Directions())*2*2 {
		t.Fatalf("allComparators returned %d states", count)
	}
	if count := len(allObservers()); count != len(cube.Faces())*2 {
		t.Fatalf("allObservers returned %d states", count)
	}
}

func TestRedstoneTicksUseRedstoneTickDuration(t *testing.T) {
	if got := redstoneTicks(1); got != time.Second/10 {
		t.Fatalf("redstoneTicks(1) = %s, want %s", got, time.Second/10)
	}
}
