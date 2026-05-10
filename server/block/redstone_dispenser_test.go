package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

func TestRedstoneDispenserStateSupport(t *testing.T) {
	if !redstoneDispenserStatesSupported("minecraft:dispenser") {
		t.Fatal("embedded block states do not support minecraft:dispenser")
	}
	if !redstoneDispenserStatesSupported("minecraft:dropper") {
		t.Fatal("embedded block states do not support minecraft:dropper")
	}
}

func TestRedstoneDispenserBlockStateCounts(t *testing.T) {
	if count := len(allDispensers()); count != len(cube.Faces())*2 {
		t.Fatalf("allDispensers returned %d states, want %d", count, len(cube.Faces())*2)
	}
	if count := len(allDroppers()); count != len(cube.Faces())*2 {
		t.Fatalf("allDroppers returned %d states, want %d", count, len(cube.Faces())*2)
	}
}

func TestRedstoneDispenserEncodeBlock(t *testing.T) {
	name, props := (Dispenser{Facing: cube.FaceEast, Triggered: true}).EncodeBlock()
	if name != "minecraft:dispenser" {
		t.Fatalf("dispenser block name = %q, want minecraft:dispenser", name)
	}
	if facing := props["facing_direction"]; facing != int32(cube.FaceEast) {
		t.Fatalf("dispenser facing_direction = %v, want %d", facing, cube.FaceEast)
	}
	if triggered := props["triggered_bit"]; triggered != uint8(1) {
		t.Fatalf("dispenser triggered_bit = %v, want 1", triggered)
	}

	name, props = (Dropper{Facing: cube.FaceNorth, Triggered: true}).EncodeBlock()
	if name != "minecraft:dropper" {
		t.Fatalf("dropper block name = %q, want minecraft:dropper", name)
	}
	if facing := props["facing_direction"]; facing != int32(cube.FaceNorth) {
		t.Fatalf("dropper facing_direction = %v, want %d", facing, cube.FaceNorth)
	}
	if triggered := props["triggered_bit"]; triggered != uint8(1) {
		t.Fatalf("dropper triggered_bit = %v, want 1", triggered)
	}
}

func TestRedstoneDispenserRegistration(t *testing.T) {
	for _, b := range []world.Block{
		Dispenser{Facing: cube.FaceSouth, Triggered: true},
		Dropper{Facing: cube.FaceUp, Triggered: true},
	} {
		name, props := b.EncodeBlock()
		got, ok := world.BlockByName(name, props)
		if !ok {
			t.Fatalf("BlockByName(%s, %#v) was not found", name, props)
		}
		if gotName, _ := got.EncodeBlock(); gotName != name {
			t.Fatalf("registered block name = %q, want %q", gotName, name)
		}
		switch b.(type) {
		case Dispenser:
			if _, ok := got.(Dispenser); !ok {
				t.Fatalf("registered block type = %T, want Dispenser", got)
			}
		case Dropper:
			if _, ok := got.(Dropper); !ok {
				t.Fatalf("registered block type = %T, want Dropper", got)
			}
		}
	}
}

func TestRedstoneDispenserPowerUpdate(t *testing.T) {
	after, changed := (Dispenser{}).RedstonePowerUpdate(cube.Pos{}, nil, 15)
	if !changed || !after.(Dispenser).Triggered {
		t.Fatalf("powered Dispenser update = %#v, %v; want triggered change", after, changed)
	}
	after, changed = (Dispenser{Triggered: true}).RedstonePowerUpdate(cube.Pos{}, nil, 0)
	if !changed || after.(Dispenser).Triggered {
		t.Fatalf("unpowered Dispenser update = %#v, %v; want untriggered change", after, changed)
	}
	after, changed = (Dropper{}).RedstonePowerUpdate(cube.Pos{}, nil, 15)
	if !changed || !after.(Dropper).Triggered {
		t.Fatalf("powered Dropper update = %#v, %v; want triggered change", after, changed)
	}
}

func TestRedstoneDispenserDropSlotRemovesOneItem(t *testing.T) {
	dropper := NewDropper()
	_ = dropper.Inventory(nil, cube.Pos{}).SetItem(4, item.NewStack(item.Stick{}, 3))

	slot, stack, ok := redstoneDispenserRandomSlot(dropper.Inventory(nil, cube.Pos{}))
	if !ok {
		t.Fatal("random slot did not find the populated slot")
	}
	if slot != 4 {
		t.Fatalf("random slot = %d, want only populated slot 4", slot)
	}
	if !redstoneDispenserDropSlot(dropper.Inventory(nil, cube.Pos{}), slot, stack, cube.Pos{}, nil, cube.FaceNorth) {
		t.Fatal("drop slot returned false")
	}
	remaining, _ := dropper.Inventory(nil, cube.Pos{}).Item(4)
	if remaining.Count() != 2 {
		t.Fatalf("remaining stack count = %d, want 2", remaining.Count())
	}
}

func TestRedstoneDispenserPowerActionSchedulesWithoutImmediateDispense(t *testing.T) {
	w := world.New()
	defer func() {
		_ = w.Close()
	}()

	var err error
	<-w.Exec(func(tx *world.Tx) {
		pos := cube.Pos{0, 1, 0}
		dispenser := NewDispenser()
		dispenser.Facing = cube.FaceEast
		tx.SetBlock(pos, dispenser, nil)
		_ = dispenser.Inventory(tx, pos).SetItem(0, item.NewStack(item.Stick{}, 2))

		if !dispenser.RedstonePowerAction(pos, tx, 0, 15) {
			err = testError("rising edge did not schedule dispenser action")
			return
		}
		remaining, _ := dispenser.Inventory(tx, pos).Item(0)
		if remaining.Count() != 2 {
			err = testError("dispenser consumed item immediately instead of waiting for scheduled tick")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRedstoneDispenserProjectileUsesRegistry(t *testing.T) {
	calls := 0
	reg := world.EntityRegistryConfig{
		Arrow: func(opts world.EntitySpawnOpts, damage float64, owner world.Entity, critical, disallowPickup, obtainArrowOnPickup bool, punchLevel int, tip any) *world.EntityHandle {
			calls++
			if owner != nil {
				t.Fatalf("dispenser arrow owner = %T, want nil", owner)
			}
			if damage != 2.0 || critical || disallowPickup || !obtainArrowOnPickup || punchLevel != 0 {
				t.Fatalf("unexpected dispenser arrow options: damage=%v critical=%v disallowPickup=%v pickup=%v punch=%d", damage, critical, disallowPickup, obtainArrowOnPickup, punchLevel)
			}
			return opts.New(redstoneDispenserTestEntityType{}, redstoneDispenserTestEntityConfig{})
		},
	}.New([]world.EntityType{redstoneDispenserTestEntityType{}})
	w := world.Config{Entities: reg}.New()
	defer func() {
		_ = w.Close()
	}()

	var err error
	<-w.Exec(func(tx *world.Tx) {
		pos := cube.Pos{0, 1, 0}
		dispenser := NewDispenser()
		dispenser.Facing = cube.FaceEast
		tx.SetBlock(pos, dispenser, nil)
		_ = dispenser.Inventory(tx, pos).SetItem(0, item.NewStack(item.Arrow{}, 2))
		if !dispenser.dispense(pos, tx) {
			err = testError("dispenser did not fire arrow")
			return
		}
		remaining, _ := dispenser.Inventory(tx, pos).Item(0)
		if remaining.Count() != 1 {
			err = testError("dispenser did not consume exactly one arrow")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("arrow factory called %d times, want 1", calls)
	}
}

func TestRedstoneDispenserEmptyInventoryDoesNotDrop(t *testing.T) {
	dropper := NewDropper()
	if _, _, ok := redstoneDispenserRandomSlot(dropper.Inventory(nil, cube.Pos{})); ok {
		t.Fatal("empty inventory unexpectedly selected a slot")
	}
	if dropper.RedstonePowerAction(cube.Pos{}, nil, 0, 15) {
		t.Fatal("empty dropper reported a redstone action")
	}
}

type redstoneDispenserTestEntityConfig struct{}

func (redstoneDispenserTestEntityConfig) Apply(*world.EntityData) {}

type redstoneDispenserTestEntityType struct{}

func (redstoneDispenserTestEntityType) Open(_ *world.Tx, handle *world.EntityHandle, _ *world.EntityData) world.Entity {
	return redstoneDispenserTestEntity{handle: handle}
}

func (redstoneDispenserTestEntityType) EncodeEntity() string {
	return "dragonfly:test_redstone_dispenser_entity"
}

func (redstoneDispenserTestEntityType) BBox(world.Entity) cube.BBox {
	return cube.BBox{}
}

func (redstoneDispenserTestEntityType) DecodeNBT(map[string]any, *world.EntityData) {}

func (redstoneDispenserTestEntityType) EncodeNBT(*world.EntityData) map[string]any {
	return nil
}

type redstoneDispenserTestEntity struct {
	handle *world.EntityHandle
}

func (e redstoneDispenserTestEntity) H() *world.EntityHandle {
	return e.handle
}

func (redstoneDispenserTestEntity) Position() mgl64.Vec3 {
	return mgl64.Vec3{}
}

func (redstoneDispenserTestEntity) Rotation() cube.Rotation {
	return cube.Rotation{}
}

func (redstoneDispenserTestEntity) Close() error {
	return nil
}

func testError(s string) error {
	return redstoneDispenserTestError(s)
}

type redstoneDispenserTestError string

func (e redstoneDispenserTestError) Error() string {
	return string(e)
}

func TestRedstoneDispenserComparatorOutput(t *testing.T) {
	dispenser := NewDispenser()
	if got := dispenser.RedstoneComparatorOutput(cube.Pos{}, nil, cube.FaceNorth); got != 0 {
		t.Fatalf("empty dispenser comparator output = %d, want 0", got)
	}
	_ = dispenser.Inventory(nil, cube.Pos{}).SetItem(0, item.NewStack(item.Stick{}, 64))
	if got := dispenser.RedstoneComparatorOutput(cube.Pos{}, nil, cube.FaceNorth); got != 2 {
		t.Fatalf("one full dispenser slot comparator output = %d, want 2", got)
	}
}
