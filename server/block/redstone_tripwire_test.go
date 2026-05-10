package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
)

func TestTripwireRedstonePower(t *testing.T) {
	if power := (String{Powered: true}).RedstonePower(cube.Pos{}, nil, cube.FaceUp); power != 15 {
		t.Fatalf("powered tripwire power = %d, want 15", power)
	}
	if power := (String{Powered: true, Disarmed: true}).RedstonePower(cube.Pos{}, nil, cube.FaceUp); power != 0 {
		t.Fatalf("disarmed tripwire power = %d, want 0", power)
	}
}

func TestTripwireHookPowerUpdate(t *testing.T) {
	after, changed := (TripwireHook{}).RedstonePowerUpdate(cube.Pos{}, nil, 15)
	if !changed || !after.(TripwireHook).Powered {
		t.Fatalf("tripwire hook update = %#v, %v; want powered change", after, changed)
	}
	if power := after.(TripwireHook).RedstonePower(cube.Pos{}, nil, cube.FaceUp); power != 15 {
		t.Fatalf("powered tripwire hook power = %d, want 15", power)
	}
}

func TestTripwireHookEncodeBlock(t *testing.T) {
	_, props := (TripwireHook{Direction: 9, Attached: true, Powered: true}).EncodeBlock()
	if direction := props["direction"]; direction != int32(3) {
		t.Fatalf("direction = %v, want 3", direction)
	}
	if attached := props["attached_bit"]; attached != uint8(1) {
		t.Fatalf("attached_bit = %v, want 1", attached)
	}
	if powered := props["powered_bit"]; powered != uint8(1) {
		t.Fatalf("powered_bit = %v, want 1", powered)
	}
}

func TestTripwireHookStateCount(t *testing.T) {
	if count := len(allTripwireHooks()); count != 16 {
		t.Fatalf("allTripwireHooks returned %d states, want 16", count)
	}
}
