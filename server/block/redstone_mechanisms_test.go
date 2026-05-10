package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

func TestBellEncodeBlock(t *testing.T) {
	name, props := (Bell{Attachment: HangingBellAttachment(), Facing: cube.East, Toggle: true}).EncodeBlock()
	if name != "minecraft:bell" {
		t.Fatalf("bell block name = %q, want minecraft:bell", name)
	}
	if attachment := props["attachment"]; attachment != "hanging" {
		t.Fatalf("bell attachment = %v, want hanging", attachment)
	}
	if direction := props["direction"]; direction != int32(3) {
		t.Fatalf("bell direction = %v, want 3", direction)
	}
	if toggle := props["toggle_bit"]; toggle != uint8(1) {
		t.Fatalf("bell toggle_bit = %v, want 1", toggle)
	}
}

func TestBellBlockStateCount(t *testing.T) {
	if count := len(allBells()); count != len(BellAttachments())*len(cube.Directions())*2 {
		t.Fatalf("allBells returned %d states", count)
	}
}

func TestBellRegisteredState(t *testing.T) {
	b, ok := world.BlockByName("minecraft:bell", map[string]any{
		"attachment": "side",
		"direction":  int32(1),
		"toggle_bit": uint8(1),
	})
	if !ok {
		t.Fatal("bell state was not registered")
	}
	if bell, ok := b.(Bell); !ok || bell.Attachment != SideBellAttachment() || bell.Facing != cube.West || !bell.Toggle {
		t.Fatalf("registered bell state = %#v, want side west toggled Bell", b)
	}
}

func TestCopperBulbRedstonePowerUpdate(t *testing.T) {
	bulb := CopperBulb{}
	after, changed := bulb.RedstonePowerUpdate(cube.Pos{}, nil, 15)
	if !changed {
		t.Fatal("copper bulb rising edge did not report a change")
	}
	bulb = after.(CopperBulb)
	if !bulb.Lit || !bulb.Powered {
		t.Fatalf("copper bulb rising edge = lit %v powered %v, want both true", bulb.Lit, bulb.Powered)
	}

	after, changed = bulb.RedstonePowerUpdate(cube.Pos{}, nil, 15)
	if changed || after.(CopperBulb) != bulb {
		t.Fatalf("copper bulb stable power changed to %#v, %v", after, changed)
	}

	after, changed = bulb.RedstonePowerUpdate(cube.Pos{}, nil, 0)
	if !changed {
		t.Fatal("copper bulb falling edge did not report a change")
	}
	bulb = after.(CopperBulb)
	if !bulb.Lit || bulb.Powered {
		t.Fatalf("copper bulb falling edge = lit %v powered %v, want lit true powered false", bulb.Lit, bulb.Powered)
	}

	after, changed = bulb.RedstonePowerUpdate(cube.Pos{}, nil, 15)
	if !changed {
		t.Fatal("copper bulb second rising edge did not report a change")
	}
	bulb = after.(CopperBulb)
	if bulb.Lit || !bulb.Powered {
		t.Fatalf("copper bulb second rising edge = lit %v powered %v, want lit false powered true", bulb.Lit, bulb.Powered)
	}
}

func TestCopperBulbEncodeBlock(t *testing.T) {
	name, props := (CopperBulb{Oxidation: WeatheredOxidation(), Waxed: true, Lit: true, Powered: true}).EncodeBlock()
	if name != "minecraft:waxed_weathered_copper_bulb" {
		t.Fatalf("copper bulb block name = %q, want minecraft:waxed_weathered_copper_bulb", name)
	}
	if lit := props["lit"]; lit != uint8(1) {
		t.Fatalf("copper bulb lit = %v, want 1", lit)
	}
	if powered := props["powered_bit"]; powered != uint8(1) {
		t.Fatalf("copper bulb powered_bit = %v, want 1", powered)
	}
}

func TestCopperBulbBlockStateCount(t *testing.T) {
	if count := len(allCopperBulbs()); count != len(OxidationTypes())*2*2*2 {
		t.Fatalf("allCopperBulbs returned %d states", count)
	}
}

func TestCopperBulbRegisteredState(t *testing.T) {
	b, ok := world.BlockByName("minecraft:waxed_weathered_copper_bulb", map[string]any{
		"lit":         uint8(1),
		"powered_bit": uint8(1),
	})
	if !ok {
		t.Fatal("waxed weathered copper bulb state was not registered")
	}
	if bulb, ok := b.(CopperBulb); !ok || bulb.Oxidation != WeatheredOxidation() || !bulb.Waxed || !bulb.Lit || !bulb.Powered {
		t.Fatalf("registered copper bulb state = %#v, want waxed weathered lit powered CopperBulb", b)
	}
}
