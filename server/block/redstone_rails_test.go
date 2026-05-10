package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
)

func TestPoweredRailRedstonePowerUpdate(t *testing.T) {
	after, changed := (PoweredRail{}).RedstonePowerUpdate(cube.Pos{}, nil, 15)
	if !changed || !after.(PoweredRail).Powered {
		t.Fatalf("powered rail update = %#v, %v; want powered change", after, changed)
	}
	_, changed = (PoweredRail{Powered: true}).RedstonePowerUpdate(cube.Pos{}, nil, 15)
	if changed {
		t.Fatal("stable powered rail reported a change")
	}
}

func TestDetectorRailPower(t *testing.T) {
	if power := (DetectorRail{}).RedstonePower(cube.Pos{}, nil, cube.FaceUp); power != 0 {
		t.Fatalf("unpowered detector rail power = %d, want 0", power)
	}
	if power := (DetectorRail{Powered: true}).RedstonePower(cube.Pos{}, nil, cube.FaceUp); power != 15 {
		t.Fatalf("powered detector rail power = %d, want 15", power)
	}
	if output := (DetectorRail{Powered: true}).RedstoneComparatorOutput(cube.Pos{}, nil, cube.FaceUp); output != 15 {
		t.Fatalf("powered detector rail comparator output = %d, want 15", output)
	}
}

func TestActivatorRailRedstonePowerUpdate(t *testing.T) {
	after, changed := (ActivatorRail{}).RedstonePowerUpdate(cube.Pos{}, nil, 15)
	if !changed || !after.(ActivatorRail).Powered {
		t.Fatalf("activator rail update = %#v, %v; want powered change", after, changed)
	}
}

func TestRailEncodeBlockClampsDirection(t *testing.T) {
	_, props := (PoweredRail{Direction: 9, Powered: true}).EncodeBlock()
	if direction := props["rail_direction"]; direction != int32(5) {
		t.Fatalf("rail_direction = %v, want 5", direction)
	}
	if powered := props["rail_data_bit"]; powered != uint8(1) {
		t.Fatalf("rail_data_bit = %v, want 1", powered)
	}
}

func TestRailStateCounts(t *testing.T) {
	if count := len(allPoweredRails()); count != 12 {
		t.Fatalf("allPoweredRails returned %d states, want 12", count)
	}
	if count := len(allDetectorRails()); count != 12 {
		t.Fatalf("allDetectorRails returned %d states, want 12", count)
	}
	if count := len(allActivatorRails()); count != 12 {
		t.Fatalf("allActivatorRails returned %d states, want 12", count)
	}
}
