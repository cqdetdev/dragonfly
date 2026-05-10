package block

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/internal/nbtconv"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/inventory"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/particle"
	"github.com/df-mc/dragonfly/server/world/sound"
	"github.com/go-gl/mathgl/mgl64"
)

var (
	_ world.RedstonePowerConsumer      = Dispenser{}
	_ world.RedstonePowerAction        = Dispenser{}
	_ world.RedstoneComparatorReadable = Dispenser{}
	_ world.ScheduledTicker            = Dispenser{}
	_ world.RedstonePowerConsumer      = Dropper{}
	_ world.RedstonePowerAction        = Dropper{}
	_ world.RedstoneComparatorReadable = Dropper{}
	_ world.ScheduledTicker            = Dropper{}
)

// Dispenser is a redstone-triggered container that dispenses one item from its inventory when powered.
type Dispenser struct {
	solid
	sourceWaterDisplacer

	// Facing is the face that items are dispensed from.
	Facing cube.Face
	// Triggered is true while the dispenser is receiving redstone power.
	Triggered bool
	// CustomName is the custom name of the dispenser. This name is displayed when the dispenser is opened, and may
	// include colour codes.
	CustomName string

	inventory *inventory.Inventory
	viewerMu  *sync.RWMutex
	viewers   map[ContainerViewer]struct{}
}

// NewDispenser creates a new initialised dispenser. The inventory is properly initialised.
func NewDispenser() Dispenser {
	inv, mu, viewers := newRedstoneDispenserInventory()
	return Dispenser{inventory: inv, viewerMu: mu, viewers: viewers}
}

// Inventory returns the inventory of the dispenser.
func (d Dispenser) Inventory(*world.Tx, cube.Pos) *inventory.Inventory {
	return d.inventory
}

// WithName returns the dispenser after applying a specific name to the block.
func (d Dispenser) WithName(a ...any) world.Item {
	d.CustomName = strings.TrimSuffix(fmt.Sprintln(a...), "\n")
	return d
}

// AddViewer adds a viewer to the dispenser, so that it is updated whenever the inventory of the dispenser changes.
func (d Dispenser) AddViewer(v ContainerViewer, _ *world.Tx, _ cube.Pos) {
	d.viewerMu.Lock()
	defer d.viewerMu.Unlock()
	d.viewers[v] = struct{}{}
}

// RemoveViewer removes a viewer from the dispenser, so slot updates are no longer sent to it.
func (d Dispenser) RemoveViewer(v ContainerViewer, _ *world.Tx, _ cube.Pos) {
	d.viewerMu.Lock()
	defer d.viewerMu.Unlock()
	delete(d.viewers, v)
}

// Activate opens the dispenser inventory.
func (d Dispenser) Activate(pos cube.Pos, _ cube.Face, tx *world.Tx, u item.User, _ *item.UseContext) bool {
	if opener, ok := u.(ContainerOpener); ok {
		opener.OpenBlockContainer(pos, tx)
		return true
	}
	return false
}

// UseOnBlock places a dispenser facing the user.
func (d Dispenser) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) (used bool) {
	pos, _, used = firstReplaceable(tx, pos, face, d)
	if !used {
		return false
	}
	d = NewDispenser()
	d.Facing = calculateFace(user, pos)

	place(tx, pos, d, user, ctx)
	return placed(ctx)
}

// BreakInfo ...
func (d Dispenser) BreakInfo() BreakInfo {
	return newBreakInfo(3.5, pickaxeHarvestable, pickaxeEffective, oneOf(Dispenser{})).withBlastResistance(17.5).withBreakHandler(func(pos cube.Pos, tx *world.Tx, _ item.User) {
		for _, i := range d.Inventory(tx, pos).Clear() {
			dropItem(tx, i, pos.Vec3Centre())
		}
	})
}

// RedstonePowerUpdate returns a dispenser with its triggered state matching the redstone power supplied.
func (d Dispenser) RedstonePowerUpdate(_ cube.Pos, _ *world.Tx, power int) (world.Block, bool) {
	triggered := power > 0
	if d.Triggered == triggered {
		return d, false
	}
	d.Triggered = triggered
	return d, true
}

// RedstonePowerAction schedules one item to be dispensed on a redstone rising edge.
func (d Dispenser) RedstonePowerAction(pos cube.Pos, tx *world.Tx, oldPower, newPower int) bool {
	if oldPower > 0 || newPower == 0 || tx == nil {
		return false
	}
	d.Triggered = true
	tx.ScheduleBlockUpdate(pos, d, redstoneTicks(2))
	d.Triggered = false
	tx.ScheduleBlockUpdate(pos, d, redstoneTicks(2))
	return true
}

// ScheduledTick executes a delayed dispenser activation.
func (d Dispenser) ScheduledTick(pos cube.Pos, tx *world.Tx, _ *rand.Rand) {
	if !d.dispense(pos, tx) {
		tx.PlaySound(pos.Vec3Centre(), sound.Click{})
	}
}

func (d Dispenser) dispense(pos cube.Pos, tx *world.Tx) bool {
	slot, stack, ok := redstoneDispenserRandomSlot(d.inventory)
	if !ok {
		return false
	}
	if d.dispenseBucket(slot, stack, pos, tx) {
		return true
	}
	if d.dispenseProjectile(slot, stack, pos, tx) {
		return true
	}
	if d.dispenseBoneMeal(slot, stack, pos, tx) {
		return true
	}
	if d.dispenseIgnition(slot, stack, pos, tx) {
		return true
	}
	if d.dispenseHoneycomb(slot, stack, pos, tx) {
		return true
	}
	if d.dispenseBottle(slot, stack, pos, tx) {
		return true
	}
	if d.dispenseTNT(slot, stack, pos, tx) {
		return true
	}
	return redstoneDispenserDropSlot(d.inventory, slot, stack, pos, tx, d.Facing)
}

func (d Dispenser) dispenseBucket(slot int, stack item.Stack, pos cube.Pos, tx *world.Tx) bool {
	if tx == nil || stack.Empty() {
		return false
	}
	bucket, ok := stack.Item().(item.Bucket)
	if !ok {
		return false
	}

	front := pos.Side(d.Facing)
	if bucket.Empty() {
		liquid, ok := tx.LiquidLoaded(front)
		if !ok || liquid.LiquidDepth() != 8 || liquid.LiquidFalling() {
			return false
		}
		filled := item.NewStack(item.Bucket{Content: item.LiquidBucketContent(liquid)}, 1)
		if !redstoneDispenserReplaceSlot(d.inventory, slot, stack, filled, pos, tx, d.Facing) {
			return false
		}
		tx.SetLiquid(front, nil)
		tx.PlaySound(front.Vec3Centre(), sound.BucketFill{Liquid: liquid})
		return true
	}

	liquid, ok := bucket.Content.Liquid()
	if !ok {
		return false
	}
	liquid = liquid.WithDepth(8, false)
	bl, ok := tx.BlockLoaded(front)
	if !ok || !canDispenserPlaceLiquid(bl, liquid) {
		return false
	}
	_ = d.inventory.SetItem(slot, item.NewStack(item.Bucket{}, 1))
	tx.SetLiquid(front, liquid)
	tx.PlaySound(front.Vec3Centre(), sound.BucketEmpty{Liquid: liquid})
	return true
}

func (d Dispenser) dispenseProjectile(slot int, stack item.Stack, pos cube.Pos, tx *world.Tx) bool {
	if tx == nil || stack.Empty() {
		return false
	}
	front := pos.Side(d.Facing)
	if _, ok := tx.BlockLoaded(front); !ok {
		return false
	}
	cfg := tx.World().EntityRegistry().Config()
	opts := redstoneDispenserProjectileOpts(pos, d.Facing, 1.1)

	switch it := stack.Item().(type) {
	case item.Arrow:
		if cfg.Arrow == nil {
			return false
		}
		if !redstoneDispenserConsumeSlot(d.inventory, slot, stack) {
			return false
		}
		tx.AddEntity(cfg.Arrow(opts, 2.0, nil, false, false, true, 0, it.Tip))
		tx.PlaySound(pos.Vec3Centre(), sound.BowShoot{})
		return true
	case item.Egg:
		if cfg.Egg == nil {
			return false
		}
		if !redstoneDispenserConsumeSlot(d.inventory, slot, stack) {
			return false
		}
		tx.AddEntity(cfg.Egg(opts, nil))
		tx.PlaySound(pos.Vec3Centre(), sound.ItemThrow{})
		return true
	case item.Snowball:
		if cfg.Snowball == nil {
			return false
		}
		if !redstoneDispenserConsumeSlot(d.inventory, slot, stack) {
			return false
		}
		tx.AddEntity(cfg.Snowball(opts, nil))
		tx.PlaySound(pos.Vec3Centre(), sound.ItemThrow{})
		return true
	case item.SplashPotion:
		if cfg.SplashPotion == nil {
			return false
		}
		if !redstoneDispenserConsumeSlot(d.inventory, slot, stack) {
			return false
		}
		tx.AddEntity(cfg.SplashPotion(opts, it.Type, nil))
		tx.PlaySound(pos.Vec3Centre(), sound.ItemThrow{})
		return true
	case item.LingeringPotion:
		if cfg.LingeringPotion == nil {
			return false
		}
		if !redstoneDispenserConsumeSlot(d.inventory, slot, stack) {
			return false
		}
		tx.AddEntity(cfg.LingeringPotion(opts, it.Type, nil))
		tx.PlaySound(pos.Vec3Centre(), sound.ItemThrow{})
		return true
	case item.BottleOfEnchanting:
		if cfg.BottleOfEnchanting == nil {
			return false
		}
		if !redstoneDispenserConsumeSlot(d.inventory, slot, stack) {
			return false
		}
		tx.AddEntity(cfg.BottleOfEnchanting(opts, nil))
		tx.PlaySound(pos.Vec3Centre(), sound.ItemThrow{})
		return true
	case item.Firework:
		if cfg.Firework == nil {
			return false
		}
		if !redstoneDispenserConsumeSlot(d.inventory, slot, stack) {
			return false
		}
		tx.AddEntity(cfg.Firework(opts, it, nil, 1.15, 0.04, false))
		tx.PlaySound(pos.Vec3Centre(), sound.FireworkLaunch{})
		return true
	}
	return false
}

func (d Dispenser) dispenseBoneMeal(slot int, stack item.Stack, pos cube.Pos, tx *world.Tx) bool {
	if tx == nil || stack.Empty() {
		return false
	}
	if _, ok := stack.Item().(item.BoneMeal); !ok {
		return false
	}
	front := pos.Side(d.Facing)
	b, ok := tx.BlockLoaded(front)
	if !ok {
		return false
	}
	affected, ok := b.(item.BoneMealAffected)
	if !ok || !affected.BoneMeal(front, tx) {
		return false
	}
	if !redstoneDispenserConsumeSlot(d.inventory, slot, stack) {
		return false
	}
	tx.AddParticle(front.Vec3(), particle.BoneMeal{})
	return true
}

func (d Dispenser) dispenseIgnition(slot int, stack item.Stack, pos cube.Pos, tx *world.Tx) bool {
	if tx == nil || stack.Empty() {
		return false
	}
	front := pos.Side(d.Facing)
	if _, ok := tx.BlockLoaded(front); !ok {
		return false
	}

	switch stack.Item().(type) {
	case item.FireCharge:
		if !redstoneDispenserIgnite(front, tx) || !redstoneDispenserConsumeSlot(d.inventory, slot, stack) {
			return false
		}
		tx.PlaySound(front.Vec3Centre(), sound.FireCharge{})
		return true
	case item.FlintAndSteel:
		if !redstoneDispenserIgnite(front, tx) || !redstoneDispenserDamageSlot(d.inventory, slot, stack, 1) {
			return false
		}
		tx.PlaySound(front.Vec3Centre(), sound.Ignite{})
		return true
	}
	return false
}

func (d Dispenser) dispenseHoneycomb(slot int, stack item.Stack, pos cube.Pos, tx *world.Tx) bool {
	if tx == nil || stack.Empty() {
		return false
	}
	if _, ok := stack.Item().(item.Honeycomb); !ok {
		return false
	}
	front := pos.Side(d.Facing)
	b, ok := tx.BlockLoaded(front)
	if !ok {
		return false
	}
	waxable, ok := b.(redstoneDispenserWaxable)
	if !ok {
		return false
	}
	res, ok := waxable.Wax(front, pos.Vec3Centre())
	if !ok || !redstoneDispenserConsumeSlot(d.inventory, slot, stack) {
		return false
	}
	tx.SetBlock(front, res, nil)
	tx.PlaySound(front.Vec3Centre(), sound.SignWaxed{})
	return true
}

func (d Dispenser) dispenseBottle(slot int, stack item.Stack, pos cube.Pos, tx *world.Tx) bool {
	if tx == nil || stack.Empty() {
		return false
	}
	if _, ok := stack.Item().(item.GlassBottle); !ok {
		return false
	}
	front := pos.Side(d.Facing)
	filled, ok := redstoneDispenserFillBottle(front, tx)
	if !ok {
		return false
	}
	return redstoneDispenserReplaceSlot(d.inventory, slot, stack, filled, pos, tx, d.Facing)
}

func (d Dispenser) dispenseTNT(slot int, stack item.Stack, pos cube.Pos, tx *world.Tx) bool {
	if tx == nil || stack.Empty() {
		return false
	}
	if _, ok := stack.Item().(TNT); !ok {
		return false
	}
	front := pos.Side(d.Facing)
	if b, ok := tx.BlockLoaded(front); !ok || !redstoneDispenserCanSpawnIn(b) {
		return false
	}
	create := tx.World().EntityRegistry().Config().TNT
	if create == nil || !redstoneDispenserConsumeSlot(d.inventory, slot, stack) {
		return false
	}
	dir := redstoneDispenserFaceVec3(d.Facing)
	opts := world.EntitySpawnOpts{
		Position: front.Vec3Centre(),
		Velocity: dir.Mul(0.2).Add(mgl64.Vec3{0, 0.2, 0}),
	}
	tx.AddEntity(create(opts, time.Second*4))
	tx.PlaySound(front.Vec3Centre(), sound.TNT{})
	return true
}

// RedstoneComparatorOutput returns the analog output for the dispenser's contents.
func (d Dispenser) RedstoneComparatorOutput(cube.Pos, *world.Tx, cube.Face) int {
	return redstoneComparatorOutputFromInventory(d.inventory)
}

// EncodeItem ...
func (Dispenser) EncodeItem() (name string, meta int16) {
	return "minecraft:dispenser", 0
}

// EncodeBlock ...
func (d Dispenser) EncodeBlock() (string, map[string]any) {
	return "minecraft:dispenser", map[string]any{
		"facing_direction": int32(d.Facing),
		"triggered_bit":    boolByte(d.Triggered),
	}
}

// EncodeNBT ...
func (d Dispenser) EncodeNBT() map[string]any {
	if d.inventory == nil {
		facing, triggered, customName := d.Facing, d.Triggered, d.CustomName
		d = NewDispenser()
		d.Facing, d.Triggered, d.CustomName = facing, triggered, customName
	}
	m := map[string]any{
		"Items": nbtconv.InvToNBT(d.inventory),
		"id":    "Dispenser",
	}
	if d.CustomName != "" {
		m["CustomName"] = d.CustomName
	}
	return m
}

// DecodeNBT ...
func (d Dispenser) DecodeNBT(data map[string]any) any {
	facing, triggered := d.Facing, d.Triggered
	d = NewDispenser()
	d.Facing, d.Triggered = facing, triggered
	d.CustomName = nbtconv.String(data, "CustomName")
	nbtconv.InvFromNBT(d.inventory, nbtconv.Slice(data, "Items"))
	return d
}

// Dropper is a redstone-triggered container that drops one item from its inventory when powered.
type Dropper struct {
	solid
	sourceWaterDisplacer

	// Facing is the face that items are dropped from.
	Facing cube.Face
	// Triggered is true while the dropper is receiving redstone power.
	Triggered bool
	// CustomName is the custom name of the dropper. This name is displayed when the dropper is opened, and may include
	// colour codes.
	CustomName string

	inventory *inventory.Inventory
	viewerMu  *sync.RWMutex
	viewers   map[ContainerViewer]struct{}
}

// NewDropper creates a new initialised dropper. The inventory is properly initialised.
func NewDropper() Dropper {
	inv, mu, viewers := newRedstoneDispenserInventory()
	return Dropper{inventory: inv, viewerMu: mu, viewers: viewers}
}

// Inventory returns the inventory of the dropper.
func (d Dropper) Inventory(*world.Tx, cube.Pos) *inventory.Inventory {
	return d.inventory
}

// WithName returns the dropper after applying a specific name to the block.
func (d Dropper) WithName(a ...any) world.Item {
	d.CustomName = strings.TrimSuffix(fmt.Sprintln(a...), "\n")
	return d
}

// AddViewer adds a viewer to the dropper, so that it is updated whenever the inventory of the dropper changes.
func (d Dropper) AddViewer(v ContainerViewer, _ *world.Tx, _ cube.Pos) {
	d.viewerMu.Lock()
	defer d.viewerMu.Unlock()
	d.viewers[v] = struct{}{}
}

// RemoveViewer removes a viewer from the dropper, so slot updates are no longer sent to it.
func (d Dropper) RemoveViewer(v ContainerViewer, _ *world.Tx, _ cube.Pos) {
	d.viewerMu.Lock()
	defer d.viewerMu.Unlock()
	delete(d.viewers, v)
}

// Activate opens the dropper inventory.
func (d Dropper) Activate(pos cube.Pos, _ cube.Face, tx *world.Tx, u item.User, _ *item.UseContext) bool {
	if opener, ok := u.(ContainerOpener); ok {
		opener.OpenBlockContainer(pos, tx)
		return true
	}
	return false
}

// UseOnBlock places a dropper facing the user.
func (d Dropper) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) (used bool) {
	pos, _, used = firstReplaceable(tx, pos, face, d)
	if !used {
		return false
	}
	d = NewDropper()
	d.Facing = calculateFace(user, pos)

	place(tx, pos, d, user, ctx)
	return placed(ctx)
}

// BreakInfo ...
func (d Dropper) BreakInfo() BreakInfo {
	return newBreakInfo(3.5, pickaxeHarvestable, pickaxeEffective, oneOf(Dropper{})).withBlastResistance(17.5).withBreakHandler(func(pos cube.Pos, tx *world.Tx, _ item.User) {
		for _, i := range d.Inventory(tx, pos).Clear() {
			dropItem(tx, i, pos.Vec3Centre())
		}
	})
}

// RedstonePowerUpdate returns a dropper with its triggered state matching the redstone power supplied.
func (d Dropper) RedstonePowerUpdate(_ cube.Pos, _ *world.Tx, power int) (world.Block, bool) {
	triggered := power > 0
	if d.Triggered == triggered {
		return d, false
	}
	d.Triggered = triggered
	return d, true
}

// RedstonePowerAction schedules one item to be dropped on a redstone rising edge.
func (d Dropper) RedstonePowerAction(pos cube.Pos, tx *world.Tx, oldPower, newPower int) bool {
	if oldPower > 0 || newPower == 0 || tx == nil {
		return false
	}
	d.Triggered = true
	tx.ScheduleBlockUpdate(pos, d, redstoneTicks(2))
	d.Triggered = false
	tx.ScheduleBlockUpdate(pos, d, redstoneTicks(2))
	return true
}

// ScheduledTick executes a delayed dropper activation.
func (d Dropper) ScheduledTick(pos cube.Pos, tx *world.Tx, _ *rand.Rand) {
	slot, stack, ok := redstoneDispenserRandomSlot(d.inventory)
	if !ok {
		tx.PlaySound(pos.Vec3Centre(), sound.Click{})
		return
	}
	if !redstoneDispenserDropSlot(d.inventory, slot, stack, pos, tx, d.Facing) {
		tx.PlaySound(pos.Vec3Centre(), sound.Click{})
	}
}

// RedstoneComparatorOutput returns the analog output for the dropper's contents.
func (d Dropper) RedstoneComparatorOutput(cube.Pos, *world.Tx, cube.Face) int {
	return redstoneComparatorOutputFromInventory(d.inventory)
}

// EncodeItem ...
func (Dropper) EncodeItem() (name string, meta int16) {
	return "minecraft:dropper", 0
}

// EncodeBlock ...
func (d Dropper) EncodeBlock() (string, map[string]any) {
	return "minecraft:dropper", map[string]any{
		"facing_direction": int32(d.Facing),
		"triggered_bit":    boolByte(d.Triggered),
	}
}

// EncodeNBT ...
func (d Dropper) EncodeNBT() map[string]any {
	if d.inventory == nil {
		facing, triggered, customName := d.Facing, d.Triggered, d.CustomName
		d = NewDropper()
		d.Facing, d.Triggered, d.CustomName = facing, triggered, customName
	}
	m := map[string]any{
		"Items": nbtconv.InvToNBT(d.inventory),
		"id":    "Dropper",
	}
	if d.CustomName != "" {
		m["CustomName"] = d.CustomName
	}
	return m
}

// DecodeNBT ...
func (d Dropper) DecodeNBT(data map[string]any) any {
	facing, triggered := d.Facing, d.Triggered
	d = NewDropper()
	d.Facing, d.Triggered = facing, triggered
	d.CustomName = nbtconv.String(data, "CustomName")
	nbtconv.InvFromNBT(d.inventory, nbtconv.Slice(data, "Items"))
	return d
}

func newRedstoneDispenserInventory() (*inventory.Inventory, *sync.RWMutex, map[ContainerViewer]struct{}) {
	m := new(sync.RWMutex)
	v := make(map[ContainerViewer]struct{}, 1)
	inv := inventory.New(9, func(slot int, _, after item.Stack) {
		m.RLock()
		defer m.RUnlock()
		for viewer := range v {
			viewer.ViewSlotChange(slot, after)
		}
	})
	return inv, m, v
}

func redstoneDispenserRandomSlot(inv *inventory.Inventory) (slot int, stack item.Stack, ok bool) {
	if inv == nil {
		return 0, item.Stack{}, false
	}
	var nonEmpty [9]int
	count := 0
	slots := inv.Slots()
	for slot, stack := range slots {
		if !stack.Empty() {
			nonEmpty[count] = slot
			count++
		}
	}
	if count == 0 {
		return 0, item.Stack{}, false
	}
	slot = nonEmpty[rand.IntN(count)]
	return slot, slots[slot], true
}

func redstoneDispenserDropSlot(inv *inventory.Inventory, slot int, stack item.Stack, pos cube.Pos, tx *world.Tx, facing cube.Face) bool {
	if inv == nil || stack.Empty() {
		return false
	}
	if tx != nil {
		if _, ok := tx.BlockLoaded(pos.Side(facing)); !ok {
			return false
		}
	}
	drop := stack.Grow(1 - stack.Count())
	_ = inv.SetItem(slot, stack.Grow(-1))
	if tx != nil {
		spawnRedstoneDispenserItem(tx, drop, pos, facing)
		tx.PlaySound(pos.Vec3Centre(), sound.Click{})
	}
	return true
}

func redstoneDispenserReplaceSlot(inv *inventory.Inventory, slot int, stack, replacement item.Stack, pos cube.Pos, tx *world.Tx, facing cube.Face) bool {
	if inv == nil || stack.Empty() || replacement.Empty() {
		return false
	}
	if stack.Count() == 1 {
		_ = inv.SetItem(slot, replacement)
		return true
	}
	if _, err := inv.AddItem(replacement); err != nil {
		if tx == nil {
			return false
		}
		spawnRedstoneDispenserItem(tx, replacement, pos, facing)
	}
	_ = inv.SetItem(slot, stack.Grow(-1))
	return true
}

func redstoneDispenserConsumeSlot(inv *inventory.Inventory, slot int, stack item.Stack) bool {
	if inv == nil || stack.Empty() {
		return false
	}
	return inv.SetItem(slot, stack.Grow(-1)) == nil
}

func redstoneDispenserDamageSlot(inv *inventory.Inventory, slot int, stack item.Stack, damage int) bool {
	if inv == nil || stack.Empty() {
		return false
	}
	return inv.SetItem(slot, stack.Damage(damage)) == nil
}

func redstoneDispenserProjectileOpts(pos cube.Pos, face cube.Face, speed float64) world.EntitySpawnOpts {
	dir := redstoneDispenserFaceVec3(face)
	spread := mgl64.Vec3{
		rand.Float64()*0.10 - 0.05,
		rand.Float64()*0.10 - 0.05,
		rand.Float64()*0.10 - 0.05,
	}
	return world.EntitySpawnOpts{
		Position: pos.Vec3Centre().Add(dir.Mul(0.7)),
		Velocity: dir.Mul(speed).Add(spread),
	}
}

func spawnRedstoneDispenserItem(tx *world.Tx, stack item.Stack, pos cube.Pos, facing cube.Face) {
	target := pos.Side(facing)
	if _, ok := tx.BlockLoaded(target); !ok {
		return
	}
	create := tx.World().EntityRegistry().Config().Item
	if create == nil {
		return
	}
	faceVelocity := redstoneDispenserFaceVec3(facing).Mul(0.25)
	opts := world.EntitySpawnOpts{
		Position: target.Vec3Centre(),
		Velocity: faceVelocity.Add(mgl64.Vec3{rand.Float64()*0.04 - 0.02, 0.1, rand.Float64()*0.04 - 0.02}),
	}
	tx.AddEntity(create(opts, stack))
}

func redstoneDispenserFaceVec3(face cube.Face) mgl64.Vec3 {
	switch face {
	case cube.FaceDown:
		return mgl64.Vec3{0, -1, 0}
	case cube.FaceUp:
		return mgl64.Vec3{0, 1, 0}
	case cube.FaceNorth:
		return mgl64.Vec3{0, 0, -1}
	case cube.FaceSouth:
		return mgl64.Vec3{0, 0, 1}
	case cube.FaceWest:
		return mgl64.Vec3{-1, 0, 0}
	case cube.FaceEast:
		return mgl64.Vec3{1, 0, 0}
	default:
		return mgl64.Vec3{}
	}
}

func redstoneDispenserIgnite(pos cube.Pos, tx *world.Tx) bool {
	if ignitable, ok := tx.Block(pos).(redstoneDispenserIgnitable); ok && ignitable.Ignite(pos, tx, nil) {
		return true
	}
	if _, ok := tx.Block(pos).(Fire); ok {
		return false
	}
	Fire{}.Start(tx, pos)
	_, ok := tx.Block(pos).(Fire)
	return ok
}

func redstoneDispenserFillBottle(pos cube.Pos, tx *world.Tx) (item.Stack, bool) {
	if filler, ok := tx.Block(pos).(redstoneDispenserBottleFiller); ok {
		res, stack, ok := filler.FillBottle()
		if !ok {
			return item.Stack{}, false
		}
		if res != tx.Block(pos) {
			tx.SetBlock(pos, res, nil)
		}
		return stack, true
	}
	if liquid, ok := tx.LiquidLoaded(pos); ok {
		if filler, ok := liquid.(redstoneDispenserBottleFiller); ok {
			_, stack, ok := filler.FillBottle()
			return stack, ok
		}
	}
	return item.Stack{}, false
}

func redstoneDispenserCanSpawnIn(b world.Block) bool {
	if _, ok := b.(Air); ok {
		return true
	}
	if r, ok := b.(Replaceable); ok {
		return r.ReplaceableBy(TNT{})
	}
	return false
}

func canDispenserPlaceLiquid(b world.Block, liquid world.Liquid) bool {
	if d, ok := b.(world.LiquidDisplacer); ok && d.CanDisplace(liquid) {
		return true
	}
	if r, ok := b.(Replaceable); ok {
		return r.ReplaceableBy(liquid)
	}
	return false
}

type redstoneDispenserIgnitable interface {
	Ignite(pos cube.Pos, tx *world.Tx, igniter world.Entity) bool
}

type redstoneDispenserBottleFiller interface {
	FillBottle() (world.Block, item.Stack, bool)
}

type redstoneDispenserWaxable interface {
	Wax(pos cube.Pos, userPos mgl64.Vec3) (world.Block, bool)
}

func allDispensers() (dispensers []world.Block) {
	for _, facing := range cube.Faces() {
		dispensers = append(dispensers, Dispenser{Facing: facing}, Dispenser{Facing: facing, Triggered: true})
	}
	return
}

func allDroppers() (droppers []world.Block) {
	for _, facing := range cube.Faces() {
		droppers = append(droppers, Dropper{Facing: facing}, Dropper{Facing: facing, Triggered: true})
	}
	return
}

func redstoneDispenserStatesSupported(name string) bool {
	for _, facing := range cube.Faces() {
		for _, triggered := range []bool{false, true} {
			if _, ok := world.BlockByName(name, map[string]any{
				"facing_direction": int32(facing),
				"triggered_bit":    boolByte(triggered),
			}); !ok {
				return false
			}
		}
	}
	return true
}
