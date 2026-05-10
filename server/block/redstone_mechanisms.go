package block

import (
	"math/rand/v2"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/sound"
	"github.com/go-gl/mathgl/mgl64"
)

// BellAttachment is the way a bell is attached to neighbouring blocks.
type BellAttachment struct {
	bellAttachment
}

type bellAttachment uint8

// StandingBellAttachment is a bell standing on top of a block.
func StandingBellAttachment() BellAttachment {
	return BellAttachment{0}
}

// HangingBellAttachment is a bell hanging from a block.
func HangingBellAttachment() BellAttachment {
	return BellAttachment{1}
}

// SideBellAttachment is a bell attached to the side of a block.
func SideBellAttachment() BellAttachment {
	return BellAttachment{2}
}

// MultipleBellAttachment is a bell attached between blocks.
func MultipleBellAttachment() BellAttachment {
	return BellAttachment{3}
}

// Uint8 returns the bell attachment as a uint8.
func (a BellAttachment) Uint8() uint8 {
	return uint8(a.bellAttachment)
}

// String returns the Bedrock block state name of the bell attachment.
func (a BellAttachment) String() string {
	switch a.bellAttachment {
	case 0:
		return "standing"
	case 1:
		return "hanging"
	case 2:
		return "side"
	case 3:
		return "multiple"
	}
	panic("unknown bell attachment")
}

// BellAttachments returns every supported bell attachment.
func BellAttachments() []BellAttachment {
	return []BellAttachment{StandingBellAttachment(), HangingBellAttachment(), SideBellAttachment(), MultipleBellAttachment()}
}

// Bell is a block that rings when activated or powered by redstone.
type Bell struct {
	empty
	transparent
	sourceWaterDisplacer

	// Attachment is how the bell is connected to nearby blocks.
	Attachment BellAttachment
	// Facing is the horizontal direction of the bell.
	Facing cube.Direction
	// Toggle is flipped whenever the bell rings.
	Toggle bool
}

// UseOnBlock places a bell using the clicked face for its attachment.
func (b Bell) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	pos, face, used := firstReplaceable(tx, pos, face, b)
	if !used {
		return false
	}
	if user != nil {
		b.Facing = user.Rotation().Direction()
	}
	switch face {
	case cube.FaceUp:
		b.Attachment = StandingBellAttachment()
	case cube.FaceDown:
		b.Attachment = HangingBellAttachment()
	default:
		b.Attachment = SideBellAttachment()
		b.Facing = face.Direction()
	}
	place(tx, pos, b, user, ctx)
	return placed(ctx)
}

// Activate rings the bell.
func (b Bell) Activate(pos cube.Pos, _ cube.Face, tx *world.Tx, _ item.User, _ *item.UseContext) bool {
	return b.Ring(pos, tx)
}

// Ring performs the bell ring side effect.
func (b Bell) Ring(pos cube.Pos, tx *world.Tx) bool {
	if tx == nil {
		return false
	}
	b.Toggle = !b.Toggle
	tx.SetBlock(pos, b, &world.SetOpts{DisableRedstoneUpdates: true})
	// TODO(redstone): Replace this with a dedicated bell block event/sound when the session layer exposes one.
	tx.PlaySound(pos.Vec3Centre(), sound.Note{Instrument: sound.Bell(), Pitch: 12})
	return true
}

// RedstonePowerAction rings the bell on a redstone rising edge.
func (b Bell) RedstonePowerAction(pos cube.Pos, tx *world.Tx, oldPower, newPower int) bool {
	if oldPower == 0 && newPower > 0 {
		return b.Ring(pos, tx)
	}
	return false
}

// BreakInfo ...
func (b Bell) BreakInfo() BreakInfo {
	return newBreakInfo(5, pickaxeHarvestable, pickaxeEffective, oneOf(b)).withBlastResistance(5)
}

// SideClosed ...
func (Bell) SideClosed(cube.Pos, cube.Pos, *world.Tx) bool {
	return false
}

// EncodeItem ...
func (Bell) EncodeItem() (name string, meta int16) {
	return "minecraft:bell", 0
}

// EncodeBlock ...
func (b Bell) EncodeBlock() (string, map[string]any) {
	return "minecraft:bell", map[string]any{
		"attachment": b.Attachment.String(),
		"direction":  bellDirection(b.Facing),
		"toggle_bit": boolByte(b.Toggle),
	}
}

func bellDirection(d cube.Direction) int32 {
	return int32(horizontalDirection(d))
}

func allBells() (bells []world.Block) {
	for _, attachment := range BellAttachments() {
		for _, facing := range cube.Directions() {
			bells = append(bells, Bell{Attachment: attachment, Facing: facing}, Bell{Attachment: attachment, Facing: facing, Toggle: true})
		}
	}
	return
}

// CopperBulb is an oxidisable light block that toggles its lit state on a redstone rising edge.
type CopperBulb struct {
	solid
	bassDrum

	// Oxidation is the level of oxidation of the copper bulb.
	Oxidation OxidationType
	// Waxed is whether the copper bulb has been waxed with honeycomb.
	Waxed bool
	// Lit is true if the bulb emits light.
	Lit bool
	// Powered is true if the bulb is currently receiving redstone power.
	Powered bool
}

// BreakInfo ...
func (c CopperBulb) BreakInfo() BreakInfo {
	return newBreakInfo(3, func(t item.Tool) bool {
		return t.ToolType() == item.TypePickaxe && t.HarvestLevel() >= item.ToolTierStone.HarvestLevel
	}, pickaxeEffective, oneOf(c)).withBlastResistance(30)
}

// LightEmissionLevel ...
func (c CopperBulb) LightEmissionLevel() uint8 {
	if c.Lit {
		return 15
	}
	return 0
}

// Wax waxes the copper bulb to stop it from oxidising further.
func (c CopperBulb) Wax(cube.Pos, mgl64.Vec3) (world.Block, bool) {
	if c.Waxed {
		return c, false
	}
	c.Waxed = true
	return c, true
}

// Strip removes wax or decreases the oxidation level of the copper bulb.
func (c CopperBulb) Strip() (world.Block, world.Sound, bool) {
	if c.Waxed {
		c.Waxed = false
		return c, sound.WaxRemoved{}, true
	} else if ot, ok := c.Oxidation.Decrease(); ok {
		c.Oxidation = ot
		return c, sound.CopperScraped{}, true
	}
	return c, nil, false
}

// CanOxidate ...
func (c CopperBulb) CanOxidate() bool {
	return !c.Waxed
}

// OxidationLevel ...
func (c CopperBulb) OxidationLevel() OxidationType {
	return c.Oxidation
}

// WithOxidationLevel ...
func (c CopperBulb) WithOxidationLevel(o OxidationType) Oxidisable {
	c.Oxidation = o
	return c
}

// RandomTick ...
func (c CopperBulb) RandomTick(pos cube.Pos, tx *world.Tx, r *rand.Rand) {
	attemptOxidation(pos, tx, r, c)
}

// RedstonePowerUpdate toggles the lit state on a redstone rising edge and tracks the powered state.
func (c CopperBulb) RedstonePowerUpdate(_ cube.Pos, _ *world.Tx, power int) (world.Block, bool) {
	powered := power > 0
	if c.Powered == powered {
		return c, false
	}
	c.Powered = powered
	if powered {
		c.Lit = !c.Lit
	}
	return c, true
}

// EncodeItem ...
func (c CopperBulb) EncodeItem() (name string, meta int16) {
	return copperBlockName("copper_bulb", c.Oxidation, c.Waxed), 0
}

// EncodeBlock ...
func (c CopperBulb) EncodeBlock() (string, map[string]any) {
	return copperBlockName("copper_bulb", c.Oxidation, c.Waxed), map[string]any{
		"lit":         boolByte(c.Lit),
		"powered_bit": boolByte(c.Powered),
	}
}

func allCopperBulbs() (bulbs []world.Block) {
	f := func(waxed bool) {
		for _, o := range OxidationTypes() {
			for _, lit := range []bool{false, true} {
				for _, powered := range []bool{false, true} {
					bulbs = append(bulbs, CopperBulb{Oxidation: o, Waxed: waxed, Lit: lit, Powered: powered})
				}
			}
		}
	}
	f(true)
	f(false)
	return
}
