package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

func TestTargetBlockPowerAndNBT(t *testing.T) {
	target := TargetBlock{Power: 20}
	if power := target.RedstonePower(cube.Pos{}, nil, cube.FaceUp); power != 15 {
		t.Fatalf("target power = %d, want 15", power)
	}
	if got := target.EncodeNBT()["power"]; got != int32(15) {
		t.Fatalf("encoded target power = %v, want 15", got)
	}
	decoded := (TargetBlock{}).DecodeNBT(map[string]any{"power": int32(7)}).(TargetBlock)
	if decoded.Power != 7 {
		t.Fatalf("decoded target power = %d, want 7", decoded.Power)
	}
	if _, stateA := (TargetBlock{}).Hash(); stateA != 0 {
		t.Fatalf("target hash state = %d, want 0", stateA)
	}
	if _, stateB := (TargetBlock{Power: 12}).Hash(); stateB != 0 {
		t.Fatalf("powered target hash state = %d, want 0", stateB)
	}
}

func TestTargetBlockHitPower(t *testing.T) {
	pos := cube.Pos{4, 5, 6}
	tests := []struct {
		name string
		hit  mgl64.Vec3
		face cube.Face
		want int
	}{
		{name: "north centre", hit: mgl64.Vec3{4.5, 5.5, 6}, face: cube.FaceNorth, want: 15},
		{name: "north midpoint", hit: mgl64.Vec3{4.75, 5.5, 6}, face: cube.FaceNorth, want: 8},
		{name: "north edge", hit: mgl64.Vec3{4, 5.5, 6}, face: cube.FaceNorth, want: 1},
		{name: "up centre", hit: mgl64.Vec3{4.5, 6, 6.5}, face: cube.FaceUp, want: 15},
	}
	for _, tt := range tests {
		if got := targetBlockHitPower(pos, tt.hit, tt.face); got != tt.want {
			t.Fatalf("%s power = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestLightningRodPowerEncodeAndStates(t *testing.T) {
	rod := LightningRod{Facing: cube.FaceEast, Powered: true}
	if power := rod.RedstonePower(cube.Pos{}, nil, cube.FaceNorth); power != 15 {
		t.Fatalf("powered lightning rod power = %d, want 15", power)
	}
	if power := (LightningRod{Facing: cube.FaceEast}).RedstonePower(cube.Pos{}, nil, cube.FaceNorth); power != 0 {
		t.Fatalf("unpowered lightning rod power = %d, want 0", power)
	}
	_, props := rod.EncodeBlock()
	if facing := props["facing_direction"]; facing != int32(cube.FaceEast) {
		t.Fatalf("facing_direction = %v, want %d", facing, cube.FaceEast)
	}
	if powered := props["powered_bit"]; powered != uint8(1) {
		t.Fatalf("powered_bit = %v, want 1", powered)
	}
	if count := len(allLightningRods()); count != len(cube.Faces())*2 {
		t.Fatalf("allLightningRods returned %d states", count)
	}
}

func TestDaylightDetectorPowerEncodeAndStates(t *testing.T) {
	if power := (DaylightDetector{}).RedstonePower(cube.Pos{}, nil, cube.FaceUp); power != 0 {
		t.Fatalf("nil tx daylight detector power = %d, want 0", power)
	}
	if power := (DaylightDetector{Inverted: true}).RedstonePower(cube.Pos{}, nil, cube.FaceUp); power != 0 {
		t.Fatalf("nil tx inverted daylight detector power = %d, want 0", power)
	}
	if power := (DaylightDetector{}).powerFromLight(12); power != 12 {
		t.Fatalf("regular daylight detector light power = %d, want 12", power)
	}
	if power := (DaylightDetector{Inverted: true}).powerFromLight(12); power != 3 {
		t.Fatalf("inverted daylight detector light power = %d, want 3", power)
	}
	if power := (DaylightDetector{}).powerFromLightAndTime(15, 18000, false, false); power != 0 {
		t.Fatalf("midnight daylight detector power = %d, want 0", power)
	}
	if power := (DaylightDetector{Inverted: true}).powerFromLightAndTime(15, 18000, false, false); power != 11 {
		t.Fatalf("midnight inverted daylight detector power = %d, want 11", power)
	}
	if power := (DaylightDetector{}).powerFromLightAndTime(15, 6000, false, false); power != 15 {
		t.Fatalf("noon daylight detector power = %d, want 15", power)
	}

	name, props := (DaylightDetector{Power: 9}).EncodeBlock()
	if name != "minecraft:daylight_detector" {
		t.Fatalf("daylight detector name = %q, want minecraft:daylight_detector", name)
	}
	if signal := props["redstone_signal"]; signal != int32(9) {
		t.Fatalf("redstone_signal = %v, want 9", signal)
	}
	name, props = (DaylightDetector{Inverted: true, Power: 20}).EncodeBlock()
	if name != "minecraft:daylight_detector_inverted" {
		t.Fatalf("inverted daylight detector name = %q, want minecraft:daylight_detector_inverted", name)
	}
	if signal := props["redstone_signal"]; signal != int32(15) {
		t.Fatalf("clamped inverted redstone_signal = %v, want 15", signal)
	}
	if count := len(allDaylightDetectors()); count != 32 {
		t.Fatalf("allDaylightDetectors returned %d states, want 32", count)
	}
	if count := len(allTargetBlocks()); count != 1 {
		t.Fatalf("allTargetBlocks returned %d states, want 1", count)
	}
}

func TestRedstoneExtraSourceRegisteredStates(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	if b, ok := world.BlockByName("minecraft:target", nil); !ok {
		t.Fatal("target block state was not registered")
	} else if _, ok := b.(TargetBlock); !ok {
		t.Fatalf("registered target state = %#v, want TargetBlock", b)
	}

	if b, ok := world.BlockByName("minecraft:lightning_rod", map[string]any{
		"facing_direction": int32(cube.FaceSouth),
		"powered_bit":      uint8(1),
	}); !ok {
		t.Fatal("powered lightning rod state was not registered")
	} else if rod, ok := b.(LightningRod); !ok || rod.Facing != cube.FaceSouth || !rod.Powered {
		t.Fatalf("registered lightning rod state = %#v, want south powered LightningRod", b)
	}

	if b, ok := world.BlockByName("minecraft:daylight_detector_inverted", map[string]any{
		"redstone_signal": int32(14),
	}); !ok {
		t.Fatal("inverted daylight detector state was not registered")
	} else if detector, ok := b.(DaylightDetector); !ok || !detector.Inverted || detector.Power != 14 {
		t.Fatalf("registered daylight detector state = %#v, want inverted power 14 DaylightDetector", b)
	}
}
