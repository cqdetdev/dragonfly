package block

import (
	"math"
	"math/rand/v2"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/model"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

const (
	targetBlockPulseTicks = 8
	daylightRefreshTicks  = 10
)

var (
	_ world.NBTer                      = TargetBlock{}
	_ world.RedstonePowerSource        = TargetBlock{}
	_ ProjectileHitter                 = TargetBlock{}
	_ world.RedstonePowerSource        = LightningRod{}
	_ world.RedstonePowerSource        = DaylightDetector{}
	_ world.RedstonePowerConsumer      = DaylightDetector{}
	_ world.RedstoneComparatorReadable = DaylightDetector{}
	_ world.ScheduledTicker            = DaylightDetector{}
)

// TargetBlock is a projectile-sensitive redstone source. Its pulse power is stored as block NBT because the
// embedded Bedrock target block state has no analog power state.
type TargetBlock struct {
	solid

	// Power is the current analog redstone power emitted by the target block.
	Power int
}

// UseOnBlock places the target block.
func (t TargetBlock) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	pos, _, used := firstReplaceable(tx, pos, face, t)
	if !used {
		return false
	}
	t.Power = 0
	place(tx, pos, t, user, ctx)
	return placed(ctx)
}

// ProjectileHit starts an analog pulse based on how close the projectile hit was to the centre of the hit face.
func (t TargetBlock) ProjectileHit(pos cube.Pos, tx *world.Tx, e world.Entity, face cube.Face) {
	if tx == nil {
		return
	}
	hitPos := pos.Vec3Centre()
	if e != nil {
		hitPos = e.Position()
	}
	t.Power = targetBlockHitPower(pos, hitPos, face)
	tx.SetBlock(pos, t, nil)
	tx.ScheduleBlockUpdate(pos, t, redstoneTicks(targetBlockPulseTicks))
}

// ScheduledTick releases an active target block pulse.
func (t TargetBlock) ScheduledTick(pos cube.Pos, tx *world.Tx, _ *rand.Rand) {
	if tx == nil || t.Power == 0 {
		return
	}
	t.Power = 0
	tx.SetBlock(pos, t, nil)
}

// RedstonePower returns the target block's current analog pulse strength.
func (t TargetBlock) RedstonePower(cube.Pos, *world.Tx, cube.Face) int {
	return redstonePower(t.Power)
}

// DecodeNBT decodes the target block's analog pulse power.
func (t TargetBlock) DecodeNBT(data map[string]any) any {
	t.Power = redstonePower(nbtInt(data["power"]))
	return t
}

// EncodeNBT encodes the target block's analog pulse power.
func (t TargetBlock) EncodeNBT() map[string]any {
	return map[string]any{"power": int32(redstonePower(t.Power))}
}

// BreakInfo ...
func (t TargetBlock) BreakInfo() BreakInfo {
	return newBreakInfo(0.5, alwaysHarvestable, hoeEffective, oneOf(t))
}

// EncodeItem ...
func (TargetBlock) EncodeItem() (name string, meta int16) {
	return "minecraft:target", 0
}

// EncodeBlock ...
func (TargetBlock) EncodeBlock() (string, map[string]any) {
	return "minecraft:target", nil
}

func allTargetBlocks() []world.Block {
	return []world.Block{TargetBlock{}}
}

func targetBlockHitPower(pos cube.Pos, hitPos mgl64.Vec3, face cube.Face) int {
	local := hitPos.Sub(pos.Vec3())
	var x, y float64
	switch face.Axis() {
	case cube.X:
		x, y = local[2]-0.5, local[1]-0.5
	case cube.Y:
		x, y = local[0]-0.5, local[2]-0.5
	default:
		x, y = local[0]-0.5, local[1]-0.5
	}
	distance := math.Sqrt(x*x + y*y)
	power := int(math.Ceil(15 * max(0, 1-distance/0.5)))
	return max(1, redstonePower(power))
}

// LightningRod is an orientable copper block that emits a short maximum-strength pulse while powered.
type LightningRod struct {
	transparent
	sourceWaterDisplacer

	// Facing is the direction the lightning rod points.
	Facing cube.Face
	// Powered is true while the lightning rod emits a redstone pulse.
	Powered bool
}

// UseOnBlock places the lightning rod pointing out of the clicked face.
func (l LightningRod) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	pos, face, used := firstReplaceable(tx, pos, face, l)
	if !used {
		return false
	}
	l.Facing = face
	place(tx, pos, l, user, ctx)
	return placed(ctx)
}

// ScheduledTick releases an active lightning rod pulse.
func (l LightningRod) ScheduledTick(pos cube.Pos, tx *world.Tx, _ *rand.Rand) {
	if tx == nil || !l.Powered {
		return
	}
	l.Powered = false
	tx.SetBlock(pos, l, nil)
}

// RedstonePower returns maximum power while the lightning rod is powered.
func (l LightningRod) RedstonePower(cube.Pos, *world.Tx, cube.Face) int {
	if l.Powered {
		return 15
	}
	return 0
}

// SideClosed ...
func (LightningRod) SideClosed(cube.Pos, cube.Pos, *world.Tx) bool {
	return false
}

// Model ...
func (l LightningRod) Model() world.BlockModel {
	return model.EndRod{Axis: l.Facing.Axis()}
}

// BreakInfo ...
func (l LightningRod) BreakInfo() BreakInfo {
	return newBreakInfo(3, pickaxeHarvestable, pickaxeEffective, oneOf(l)).withBlastResistance(6)
}

// EncodeItem ...
func (LightningRod) EncodeItem() (name string, meta int16) {
	return "minecraft:lightning_rod", 0
}

// EncodeBlock ...
func (l LightningRod) EncodeBlock() (string, map[string]any) {
	return "minecraft:lightning_rod", map[string]any{
		"facing_direction": int32(l.Facing),
		"powered_bit":      boolByte(l.Powered),
	}
}

func allLightningRods() (rods []world.Block) {
	for _, face := range cube.Faces() {
		rods = append(rods, LightningRod{Facing: face}, LightningRod{Facing: face, Powered: true})
	}
	return
}

// DaylightDetector is an analog redstone source whose output follows the skylight at its position.
type DaylightDetector struct {
	transparent
	sourceWaterDisplacer

	// Inverted is true for an inverted daylight detector.
	Inverted bool
	// Power is the last stored analog output used for the Bedrock block state.
	Power int
}

// UseOnBlock places the daylight detector.
func (d DaylightDetector) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	pos, _, used := firstReplaceable(tx, pos, face, d)
	if !used {
		return false
	}
	d.Power = d.daylightPower(pos, tx)
	place(tx, pos, d, user, ctx)
	if placed(ctx) {
		tx.ScheduleBlockUpdate(pos, d, redstoneTicks(daylightRefreshTicks))
	}
	return placed(ctx)
}

// Activate toggles between regular and inverted daylight detector output.
func (d DaylightDetector) Activate(pos cube.Pos, _ cube.Face, tx *world.Tx, _ item.User, _ *item.UseContext) bool {
	if tx == nil {
		return false
	}
	d.Inverted = !d.Inverted
	d.Power = d.daylightPower(pos, tx)
	tx.SetBlock(pos, d, nil)
	tx.ScheduleBlockUpdate(pos, d, redstoneTicks(daylightRefreshTicks))
	return true
}

// ScheduledTick refreshes the stored Bedrock redstone_signal state as world time changes.
func (d DaylightDetector) ScheduledTick(pos cube.Pos, tx *world.Tx, _ *rand.Rand) {
	if tx == nil {
		return
	}
	power := d.daylightPower(pos, tx)
	if d.Power != power {
		d.Power = power
		tx.SetBlock(pos, d, nil)
	}
	tx.ScheduleBlockUpdate(pos, d, redstoneTicks(daylightRefreshTicks))
}

// RedstonePower returns the daylight detector's analog output based on skylight.
func (d DaylightDetector) RedstonePower(pos cube.Pos, tx *world.Tx, _ cube.Face) int {
	return d.daylightPower(pos, tx)
}

// RedstonePowerUpdate refreshes the stored Bedrock redstone_signal state when a redstone update reaches the detector.
func (d DaylightDetector) RedstonePowerUpdate(pos cube.Pos, tx *world.Tx, _ int) (world.Block, bool) {
	power := d.daylightPower(pos, tx)
	if d.Power == power {
		return d, false
	}
	d.Power = power
	return d, true
}

// RedstoneComparatorOutput exposes the detector's current analog output to comparators.
func (d DaylightDetector) RedstoneComparatorOutput(pos cube.Pos, tx *world.Tx, _ cube.Face) int {
	return d.daylightPower(pos, tx)
}

// SideClosed ...
func (DaylightDetector) SideClosed(cube.Pos, cube.Pos, *world.Tx) bool {
	return false
}

// Model ...
func (DaylightDetector) Model() world.BlockModel {
	return daylightDetectorModel{}
}

// BreakInfo ...
func (d DaylightDetector) BreakInfo() BreakInfo {
	return newBreakInfo(0.2, alwaysHarvestable, nothingEffective, oneOf(d))
}

// EncodeItem ...
func (DaylightDetector) EncodeItem() (name string, meta int16) {
	return "minecraft:daylight_detector", 0
}

// EncodeBlock ...
func (d DaylightDetector) EncodeBlock() (string, map[string]any) {
	name := "minecraft:daylight_detector"
	if d.Inverted {
		name = "minecraft:daylight_detector_inverted"
	}
	return name, map[string]any{"redstone_signal": int32(redstonePower(d.Power))}
}

func (d DaylightDetector) daylightPower(pos cube.Pos, tx *world.Tx) int {
	if tx == nil {
		return 0
	}
	return d.powerFromLightAndTime(tx.SkyLight(pos), tx.World().Time(), tx.RainingAt(pos) || tx.SnowingAt(pos), tx.ThunderingAt(pos))
}

func (d DaylightDetector) powerFromLight(light uint8) int {
	return d.powerFromLightAndTime(light, 6000, false, false)
}

func (d DaylightDetector) powerFromLightAndTime(light uint8, dayTime int, raining, thundering bool) int {
	power := daylightClearSignal(dayTime)
	if thundering {
		power = int(math.Round(float64(power) * 10 / 15))
	} else if raining {
		power = int(math.Round(float64(power) * 12 / 15))
	}
	power = int(math.Round(float64(power) * float64(redstonePower(int(light))) / 15))
	if d.Inverted {
		internal := max(4, power)
		if light == 0 {
			internal = 0
		}
		return redstonePower(15 - internal)
	}
	return redstonePower(power)
}

func daylightClearSignal(dayTime int) int {
	t := dayTime % world.TimeFull
	if t < 0 {
		t += world.TimeFull
	}
	switch {
	case t >= 4295 && t <= 7705:
		return 15
	case (t >= 3176 && t <= 4294) || (t >= 7706 && t <= 8825):
		return 14
	case (t >= 2445 && t <= 3175) || (t >= 8826 && t <= 9556):
		return 13
	case (t >= 1866 && t <= 2444) || (t >= 9557 && t <= 10135):
		return 12
	case (t >= 1372 && t <= 1865) || (t >= 10136 && t <= 10628):
		return 11
	case (t >= 934 && t <= 1371) || (t >= 10629 && t <= 11066):
		return 10
	case (t >= 536 && t <= 933) || (t >= 11067 && t <= 11464):
		return 9
	case (t >= 167 && t <= 535) || (t >= 11465 && t <= 11834):
		return 8
	case t <= 166 || (t >= 11835 && t <= 12040) || t >= 23961:
		return 7
	case (t >= 23768 && t <= 23960) || (t >= 12041 && t <= 12232):
		return 6
	case (t >= 23530 && t <= 23767) || (t >= 12233 && t <= 12470):
		return 5
	case (t >= 23297 && t <= 23529) || (t >= 12471 && t <= 12704):
		return 4
	case (t >= 23071 && t <= 23296) || (t >= 12705 && t <= 12930):
		return 3
	case (t >= 22782 && t <= 23070) || (t >= 12931 && t <= 13218):
		return 2
	case (t >= 22331 && t <= 22781) || (t >= 13219 && t <= 13669):
		return 1
	default:
		return 0
	}
}

func allDaylightDetectors() (detectors []world.Block) {
	for _, inverted := range []bool{false, true} {
		for power := 0; power <= 15; power++ {
			detectors = append(detectors, DaylightDetector{Inverted: inverted, Power: power})
		}
	}
	return
}

type daylightDetectorModel struct{}

func (daylightDetectorModel) BBox(cube.Pos, world.BlockSource) []cube.BBox {
	return []cube.BBox{cube.Box(0, 0, 0, 1, 0.375, 1)}
}

func (daylightDetectorModel) FaceSolid(cube.Pos, cube.Face, world.BlockSource) bool {
	return false
}

func nbtInt(v any) int {
	switch v := v.(type) {
	case int:
		return v
	case int8:
		return int(v)
	case int16:
		return int(v)
	case int32:
		return int(v)
	case int64:
		return int(v)
	case uint8:
		return int(v)
	case uint16:
		return int(v)
	case uint32:
		return int(v)
	case uint64:
		return int(v)
	default:
		return 0
	}
}
