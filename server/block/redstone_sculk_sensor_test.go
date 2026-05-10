package block

import (
	"fmt"
	"os"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

func TestMain(m *testing.M) {
	world.DefaultBlockRegistry.Finalize()
	os.Exit(m.Run())
}

func TestSculkSensorEncodeAndStateCounts(t *testing.T) {
	name, props := (SculkSensor{Phase: SculkSensorPhaseActive}).EncodeBlock()
	if name != "minecraft:sculk_sensor" {
		t.Fatalf("sculk sensor block name = %q, want minecraft:sculk_sensor", name)
	}
	if phase := props["sculk_sensor_phase"]; phase != int32(SculkSensorPhaseActive) {
		t.Fatalf("sculk_sensor_phase = %v, want %d", phase, SculkSensorPhaseActive)
	}

	name, props = (CalibratedSculkSensor{Facing: cube.West, Phase: SculkSensorPhaseCooldown}).EncodeBlock()
	if name != "minecraft:calibrated_sculk_sensor" {
		t.Fatalf("calibrated sculk sensor block name = %q, want minecraft:calibrated_sculk_sensor", name)
	}
	if direction := props["minecraft:cardinal_direction"]; direction != "west" {
		t.Fatalf("minecraft:cardinal_direction = %v, want west", direction)
	}
	if phase := props["sculk_sensor_phase"]; phase != int32(SculkSensorPhaseCooldown) {
		t.Fatalf("calibrated sculk_sensor_phase = %v, want %d", phase, SculkSensorPhaseCooldown)
	}

	if count := len(allSculkSensors()); count != 3 {
		t.Fatalf("allSculkSensors returned %d states, want 3", count)
	}
	if count := len(allCalibratedSculkSensors()); count != len(cube.Directions())*3 {
		t.Fatalf("allCalibratedSculkSensors returned %d states, want %d", count, len(cube.Directions())*3)
	}
}

func TestSculkSensorRegistration(t *testing.T) {
	tests := []struct {
		name  string
		block world.Block
		check func(world.Block) bool
	}{
		{
			name:  "active sculk sensor",
			block: SculkSensor{Phase: SculkSensorPhaseActive},
			check: func(b world.Block) bool {
				got, ok := b.(SculkSensor)
				return ok && got.Phase == SculkSensorPhaseActive
			},
		},
		{
			name:  "east calibrated cooldown sensor",
			block: CalibratedSculkSensor{Facing: cube.East, Phase: SculkSensorPhaseCooldown},
			check: func(b world.Block) bool {
				got, ok := b.(CalibratedSculkSensor)
				return ok && got.Facing == cube.East && got.Phase == SculkSensorPhaseCooldown
			},
		},
	}
	for _, test := range tests {
		name, props := test.block.EncodeBlock()
		got, ok := world.BlockByName(name, props)
		if !ok {
			t.Fatalf("%s BlockByName(%s, %#v) was not found", test.name, name, props)
		}
		if !test.check(got) {
			t.Fatalf("%s registered block = %#v", test.name, got)
		}
	}
}

func TestSculkSensorPowerNBTAndHash(t *testing.T) {
	sensor := SculkSensor{Phase: SculkSensorPhaseActive, Power: 20, Frequency: 4}
	if power := sensor.RedstonePower(cube.Pos{}, nil, cube.FaceUp); power != 15 {
		t.Fatalf("active sculk sensor power = %d, want 15", power)
	}
	if power := (SculkSensor{Phase: SculkSensorPhaseActive}).RedstonePower(cube.Pos{}, nil, cube.FaceUp); power != 15 {
		t.Fatalf("active sculk sensor default power = %d, want 15", power)
	}
	if power := (SculkSensor{Phase: SculkSensorPhaseCooldown, Power: 15}).RedstonePower(cube.Pos{}, nil, cube.FaceUp); power != 0 {
		t.Fatalf("cooldown sculk sensor power = %d, want 0", power)
	}
	if output := sensor.RedstoneComparatorOutput(cube.Pos{}, nil, cube.FaceUp); output != 4 {
		t.Fatalf("sculk sensor comparator output = %d, want 4", output)
	}
	if got := sensor.EncodeNBT()["power"]; got != int32(15) {
		t.Fatalf("encoded sculk sensor power = %v, want 15", got)
	}
	if got := sensor.EncodeNBT()["frequency"]; got != int32(4) {
		t.Fatalf("encoded sculk sensor frequency = %v, want 4", got)
	}
	decoded := (SculkSensor{}).DecodeNBT(map[string]any{"power": int32(7), "frequency": int32(12)}).(SculkSensor)
	if decoded.Power != 7 || decoded.Frequency != 12 {
		t.Fatalf("decoded sculk sensor = %#v, want power 7 frequency 12", decoded)
	}

	_, inactiveHash := (SculkSensor{Phase: SculkSensorPhaseInactive}).Hash()
	_, activeHash := (SculkSensor{Phase: SculkSensorPhaseActive}).Hash()
	if inactiveHash == activeHash {
		t.Fatal("sculk sensor hash should include Phase")
	}
	_, northHash := (CalibratedSculkSensor{Facing: cube.North}).Hash()
	_, eastHash := (CalibratedSculkSensor{Facing: cube.East}).Hash()
	if northHash == eastHash {
		t.Fatal("calibrated sculk sensor hash should include Facing")
	}
}

func TestSculkSensorVibrationActivationAndDecay(t *testing.T) {
	w := world.New()
	defer func() {
		_ = w.Close()
	}()

	var err error
	<-w.Exec(func(tx *world.Tx) {
		pos := cube.Pos{0, 0, 0}
		tx.SetBlock(pos, SculkSensor{}, nil)
		if !TriggerSculkSensorVibration(pos, tx, SculkVibration{Power: 9, Frequency: 6}) {
			err = fmt.Errorf("TriggerSculkSensorVibration returned false")
			return
		}
		active := tx.Block(pos).(SculkSensor)
		if active.Phase != SculkSensorPhaseActive || active.Power != 9 || active.Frequency != 6 {
			err = fmt.Errorf("active sculk sensor = %#v, want phase active power 9 frequency 6", active)
			return
		}
		if active.RedstonePower(pos, tx, cube.FaceNorth) != 9 {
			err = fmt.Errorf("active sculk sensor redstone power = %d, want 9", active.RedstonePower(pos, tx, cube.FaceNorth))
			return
		}
		if TriggerSculkSensorVibration(pos, tx, SculkVibration{Power: 15, Frequency: 15}) {
			err = fmt.Errorf("active sculk sensor accepted another vibration")
			return
		}

		active.ScheduledTick(pos, tx, nil)
		cooldown := tx.Block(pos).(SculkSensor)
		if cooldown.Phase != SculkSensorPhaseCooldown || cooldown.Power != 0 || cooldown.Frequency != 6 {
			err = fmt.Errorf("cooldown sculk sensor = %#v, want phase cooldown power 0 frequency 6", cooldown)
			return
		}
		if cooldown.RedstonePower(pos, tx, cube.FaceNorth) != 0 {
			err = fmt.Errorf("cooldown sculk sensor emitted power %d, want 0", cooldown.RedstonePower(pos, tx, cube.FaceNorth))
			return
		}

		cooldown.ScheduledTick(pos, tx, nil)
		inactive := tx.Block(pos).(SculkSensor)
		if inactive.Phase != SculkSensorPhaseInactive || inactive.Power != 0 || inactive.Frequency != 0 {
			err = fmt.Errorf("inactive sculk sensor = %#v, want phase inactive power 0 frequency 0", inactive)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCalibratedSculkSensorFilter(t *testing.T) {
	w := world.New()
	defer func() {
		_ = w.Close()
	}()

	var err error
	<-w.Exec(func(tx *world.Tx) {
		pos := cube.Pos{0, 0, 0}
		sensor := CalibratedSculkSensor{Facing: cube.North}
		tx.SetBlock(pos, sensor, nil)
		tx.SetBlock(pos.Side(cube.FaceSouth), RedstoneBlock{}, nil)

		if input := sensor.CalibrationInput(pos, tx); input != 15 {
			err = fmt.Errorf("calibrated sculk sensor input = %d, want 15", input)
			return
		}
		if TriggerSculkSensorVibration(pos, tx, SculkVibration{Power: 12, Frequency: 7}) {
			err = fmt.Errorf("calibrated sculk sensor accepted frequency 7 with filter 15")
			return
		}
		if !TriggerSculkSensorVibration(pos, tx, SculkVibration{Power: 12, Frequency: 15}) {
			err = fmt.Errorf("calibrated sculk sensor rejected frequency 15 with filter 15")
			return
		}
		active := tx.Block(pos).(CalibratedSculkSensor)
		if active.Phase != SculkSensorPhaseActive || active.Power != 12 || active.Frequency != 15 {
			err = fmt.Errorf("active calibrated sculk sensor = %#v, want phase active power 12 frequency 15", active)
			return
		}
		if output := active.RedstoneComparatorOutput(pos, tx, cube.FaceNorth); output != 15 {
			err = fmt.Errorf("active calibrated comparator output = %d, want 15", output)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}
