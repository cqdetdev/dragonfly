package block

import (
	"math/rand/v2"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

const (
	// SculkSensorPhaseInactive means the sensor is waiting for a vibration.
	SculkSensorPhaseInactive = iota
	// SculkSensorPhaseActive means the sensor is emitting redstone power.
	SculkSensorPhaseActive
	// SculkSensorPhaseCooldown means the sensor has just emitted a pulse and is temporarily inactive.
	SculkSensorPhaseCooldown

	sculkSensorActiveTicks   = 30
	sculkSensorCooldownTicks = 10
)

var (
	_ world.NBTer                      = SculkSensor{}
	_ world.ScheduledTicker            = SculkSensor{}
	_ world.RedstonePowerSource        = SculkSensor{}
	_ world.RedstoneComparatorReadable = SculkSensor{}

	_ world.NBTer                      = CalibratedSculkSensor{}
	_ world.ScheduledTicker            = CalibratedSculkSensor{}
	_ world.RedstonePowerSource        = CalibratedSculkSensor{}
	_ world.RedstoneComparatorReadable = CalibratedSculkSensor{}
)

// SculkVibration describes a detected vibration that can activate a sculk sensor. Dragonfly does not yet have a
// game-event bus, so callers that detect vibrations may pass the analog redstone power and vibration frequency here.
type SculkVibration struct {
	// Power is the redstone power emitted while the sensor is active. Values outside 1-15 are clamped; values <= 0
	// default to 15 for callers that only need a digital pulse.
	Power int
	// Frequency is the vibration frequency exposed to comparators and used by calibrated sensors with a side filter.
	Frequency int
}

// TriggerSculkSensorVibration activates the sculk sensor at pos from a detected vibration. It returns false if pos is
// not a sculk sensor, if the sensor is cooling down or active, or if a calibrated sensor rejects the vibration.
func TriggerSculkSensorVibration(pos cube.Pos, tx *world.Tx, vibration SculkVibration) bool {
	if tx == nil {
		return false
	}
	switch b := tx.Block(pos).(type) {
	case SculkSensor:
		return b.ActivateVibration(pos, tx, vibration)
	case CalibratedSculkSensor:
		return b.ActivateVibration(pos, tx, vibration)
	default:
		return false
	}
}

// SculkSensor is a vibration-sensitive redstone source.
type SculkSensor struct {
	transparent
	sourceWaterDisplacer

	// Phase is the current Bedrock sculk_sensor_phase value.
	Phase int
	// Power is transient analog redstone power stored in block NBT because Bedrock stores only the phase as a block
	// state.
	Power int
	// Frequency is the last detected vibration frequency exposed to comparators.
	Frequency int
}

// UseOnBlock places a sculk sensor.
func (s SculkSensor) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	pos, _, used := firstReplaceable(tx, pos, face, s)
	if !used {
		return false
	}
	s.Phase, s.Power, s.Frequency = SculkSensorPhaseInactive, 0, 0
	place(tx, pos, s, user, ctx)
	return placed(ctx)
}

// ActivateVibration starts a sculk sensor pulse from a detected vibration.
func (s SculkSensor) ActivateVibration(pos cube.Pos, tx *world.Tx, vibration SculkVibration) bool {
	if tx == nil || sculkSensorPhase(s.Phase) != SculkSensorPhaseInactive {
		return false
	}
	s.Phase = SculkSensorPhaseActive
	s.Power = vibration.redstonePower()
	s.Frequency = vibration.frequency()
	tx.SetBlock(pos, s, nil)
	tx.ScheduleBlockUpdate(pos, s, redstoneTicks(sculkSensorActiveTicks))
	return true
}

// ScheduledTick advances the sensor from active to cooldown, or from cooldown back to inactive.
func (s SculkSensor) ScheduledTick(pos cube.Pos, tx *world.Tx, _ *rand.Rand) {
	if tx == nil {
		return
	}
	switch sculkSensorPhase(s.Phase) {
	case SculkSensorPhaseActive:
		s.Phase = SculkSensorPhaseCooldown
		s.Power = 0
		tx.SetBlock(pos, s, nil)
		tx.ScheduleBlockUpdate(pos, s, redstoneTicks(sculkSensorCooldownTicks))
	case SculkSensorPhaseCooldown:
		s.Phase = SculkSensorPhaseInactive
		s.Power, s.Frequency = 0, 0
		tx.SetBlock(pos, s, nil)
	}
}

// RedstonePower returns the sensor's analog power while active.
func (s SculkSensor) RedstonePower(cube.Pos, *world.Tx, cube.Face) int {
	return sculkSensorActivePower(s.Phase, s.Power)
}

// RedstoneComparatorOutput returns the last detected vibration frequency.
func (s SculkSensor) RedstoneComparatorOutput(cube.Pos, *world.Tx, cube.Face) int {
	return redstonePower(s.Frequency)
}

// DecodeNBT decodes transient analog sculk sensor data.
func (s SculkSensor) DecodeNBT(data map[string]any) any {
	s.Power = redstonePower(nbtInt(data["power"]))
	s.Frequency = redstonePower(nbtInt(data["frequency"]))
	return s
}

// EncodeNBT encodes transient analog sculk sensor data.
func (s SculkSensor) EncodeNBT() map[string]any {
	return map[string]any{"power": int32(redstonePower(s.Power)), "frequency": int32(redstonePower(s.Frequency))}
}

// SideClosed ...
func (SculkSensor) SideClosed(cube.Pos, cube.Pos, *world.Tx) bool {
	return false
}

// Model ...
func (SculkSensor) Model() world.BlockModel {
	return sculkSensorModel{}
}

// BreakInfo ...
func (s SculkSensor) BreakInfo() BreakInfo {
	return newBreakInfo(1.5, alwaysHarvestable, hoeEffective, oneOf(s))
}

// EncodeItem ...
func (SculkSensor) EncodeItem() (name string, meta int16) {
	return "minecraft:sculk_sensor", 0
}

// EncodeBlock ...
func (s SculkSensor) EncodeBlock() (string, map[string]any) {
	return "minecraft:sculk_sensor", map[string]any{"sculk_sensor_phase": int32(sculkSensorPhase(s.Phase))}
}

func allSculkSensors() (sensors []world.Block) {
	for phase := SculkSensorPhaseInactive; phase <= SculkSensorPhaseCooldown; phase++ {
		sensors = append(sensors, SculkSensor{Phase: phase})
	}
	return
}

// CalibratedSculkSensor is a directional sculk sensor that may filter vibrations by side redstone input.
type CalibratedSculkSensor struct {
	transparent
	sourceWaterDisplacer

	// Facing is the sensor's cardinal direction.
	Facing cube.Direction
	// Phase is the current Bedrock sculk_sensor_phase value.
	Phase int
	// Power is transient analog redstone power stored in block NBT because Bedrock stores only the phase as a block
	// state.
	Power int
	// Frequency is the last detected vibration frequency exposed to comparators.
	Frequency int
}

// UseOnBlock places a calibrated sculk sensor facing the user's horizontal direction.
func (s CalibratedSculkSensor) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	pos, _, used := firstReplaceable(tx, pos, face, s)
	if !used {
		return false
	}
	if user != nil {
		s.Facing = user.Rotation().Direction()
	}
	s.Phase, s.Power, s.Frequency = SculkSensorPhaseInactive, 0, 0
	place(tx, pos, s, user, ctx)
	return placed(ctx)
}

// ActivateVibration starts a calibrated sculk sensor pulse if its side filter accepts the vibration.
func (s CalibratedSculkSensor) ActivateVibration(pos cube.Pos, tx *world.Tx, vibration SculkVibration) bool {
	if tx == nil || sculkSensorPhase(s.Phase) != SculkSensorPhaseInactive || !s.AcceptsVibration(pos, tx, vibration) {
		return false
	}
	s.Phase = SculkSensorPhaseActive
	s.Power = vibration.redstonePower()
	s.Frequency = vibration.frequency()
	tx.SetBlock(pos, s, nil)
	tx.ScheduleBlockUpdate(pos, s, redstoneTicks(sculkSensorActiveTicks))
	return true
}

// AcceptsVibration checks the calibrated sensor's side input filter. A zero side input accepts all frequencies.
func (s CalibratedSculkSensor) AcceptsVibration(pos cube.Pos, tx *world.Tx, vibration SculkVibration) bool {
	filter := s.CalibrationInput(pos, tx)
	frequency := vibration.frequency()
	return filter == 0 || (frequency != 0 && frequency == filter)
}

// CalibrationInput returns the current filter input power read from the side opposite the sensor's facing.
func (s CalibratedSculkSensor) CalibrationInput(pos cube.Pos, tx *world.Tx) int {
	if tx == nil {
		return 0
	}
	return redstonePower(tx.RedstonePowerFrom(pos, sculkSensorDirection(s.Facing).Opposite().Face()))
}

// ScheduledTick advances the sensor from active to cooldown, or from cooldown back to inactive.
func (s CalibratedSculkSensor) ScheduledTick(pos cube.Pos, tx *world.Tx, _ *rand.Rand) {
	if tx == nil {
		return
	}
	switch sculkSensorPhase(s.Phase) {
	case SculkSensorPhaseActive:
		s.Phase = SculkSensorPhaseCooldown
		s.Power = 0
		tx.SetBlock(pos, s, nil)
		tx.ScheduleBlockUpdate(pos, s, redstoneTicks(sculkSensorCooldownTicks))
	case SculkSensorPhaseCooldown:
		s.Phase = SculkSensorPhaseInactive
		s.Power, s.Frequency = 0, 0
		tx.SetBlock(pos, s, nil)
	}
}

// RedstonePower returns the sensor's analog power while active.
func (s CalibratedSculkSensor) RedstonePower(cube.Pos, *world.Tx, cube.Face) int {
	return sculkSensorActivePower(s.Phase, s.Power)
}

// RedstoneComparatorOutput returns the last detected vibration frequency.
func (s CalibratedSculkSensor) RedstoneComparatorOutput(cube.Pos, *world.Tx, cube.Face) int {
	return redstonePower(s.Frequency)
}

// DecodeNBT decodes transient analog sculk sensor data.
func (s CalibratedSculkSensor) DecodeNBT(data map[string]any) any {
	s.Power = redstonePower(nbtInt(data["power"]))
	s.Frequency = redstonePower(nbtInt(data["frequency"]))
	return s
}

// EncodeNBT encodes transient analog sculk sensor data.
func (s CalibratedSculkSensor) EncodeNBT() map[string]any {
	return map[string]any{"power": int32(redstonePower(s.Power)), "frequency": int32(redstonePower(s.Frequency))}
}

// SideClosed ...
func (CalibratedSculkSensor) SideClosed(cube.Pos, cube.Pos, *world.Tx) bool {
	return false
}

// Model ...
func (CalibratedSculkSensor) Model() world.BlockModel {
	return sculkSensorModel{}
}

// BreakInfo ...
func (s CalibratedSculkSensor) BreakInfo() BreakInfo {
	return newBreakInfo(1.5, alwaysHarvestable, hoeEffective, oneOf(s))
}

// EncodeItem ...
func (CalibratedSculkSensor) EncodeItem() (name string, meta int16) {
	return "minecraft:calibrated_sculk_sensor", 0
}

// EncodeBlock ...
func (s CalibratedSculkSensor) EncodeBlock() (string, map[string]any) {
	return "minecraft:calibrated_sculk_sensor", map[string]any{
		"minecraft:cardinal_direction": sculkSensorDirection(s.Facing).String(),
		"sculk_sensor_phase":           int32(sculkSensorPhase(s.Phase)),
	}
}

func allCalibratedSculkSensors() (sensors []world.Block) {
	for _, facing := range cube.Directions() {
		for phase := SculkSensorPhaseInactive; phase <= SculkSensorPhaseCooldown; phase++ {
			sensors = append(sensors, CalibratedSculkSensor{Facing: facing, Phase: phase})
		}
	}
	return
}

func (v SculkVibration) redstonePower() int {
	if v.Power <= 0 {
		return 15
	}
	return redstonePower(v.Power)
}

func (v SculkVibration) frequency() int {
	return redstonePower(v.Frequency)
}

func sculkSensorActivePower(phase, power int) int {
	if sculkSensorPhase(phase) != SculkSensorPhaseActive {
		return 0
	}
	if power <= 0 {
		return 15
	}
	return redstonePower(power)
}

func sculkSensorPhase(phase int) int {
	if phase < SculkSensorPhaseInactive || phase > SculkSensorPhaseCooldown {
		return SculkSensorPhaseInactive
	}
	return phase
}

func sculkSensorDirection(d cube.Direction) cube.Direction {
	for _, direction := range cube.Directions() {
		if d == direction {
			return d
		}
	}
	return cube.South
}

type sculkSensorModel struct{}

func (sculkSensorModel) BBox(cube.Pos, world.BlockSource) []cube.BBox {
	return []cube.BBox{cube.Box(0, 0, 0, 1, 0.5, 1)}
}

func (sculkSensorModel) FaceSolid(cube.Pos, cube.Face, world.BlockSource) bool {
	return false
}
