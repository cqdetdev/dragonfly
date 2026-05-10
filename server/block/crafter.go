package block

import (
	"fmt"
	"strings"
	"sync"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/internal/nbtconv"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/inventory"
	"github.com/df-mc/dragonfly/server/item/recipe"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

var (
	_ world.RedstonePowerConsumer      = Crafter{}
	_ world.RedstonePowerAction        = Crafter{}
	_ world.RedstoneComparatorReadable = Crafter{}
)

// Crafter is a redstone-triggered 3x3 crafting container.
// The empty value of Crafter is not valid. It must be created using block.NewCrafter().
type Crafter struct {
	solid
	sourceWaterDisplacer

	// Orientation is the crafter's output face and top orientation.
	Orientation CrafterOrientation
	// Triggered is true while the crafter is receiving redstone power.
	Triggered bool
	// Crafting is true while the crafter is displaying its crafting state.
	Crafting bool
	// DisabledSlots is a bitset of the crafter's disabled slots. Bit 0 is the top-left slot and bit 8 is the
	// bottom-right slot.
	DisabledSlots uint16
	// CustomName is the custom name of the crafter. This name is displayed when the crafter is opened, and may
	// include colour codes.
	CustomName string

	inventory *inventory.Inventory
	viewerMu  *sync.RWMutex
	viewers   map[ContainerViewer]struct{}
}

// NewCrafter creates a new initialised crafter. The inventory is properly initialised.
func NewCrafter() Crafter {
	m := new(sync.RWMutex)
	v := make(map[ContainerViewer]struct{}, 1)
	return Crafter{
		inventory: inventory.New(9, func(slot int, _, after item.Stack) {
			m.RLock()
			defer m.RUnlock()
			for viewer := range v {
				viewer.ViewSlotChange(slot, after)
			}
		}),
		viewerMu: m,
		viewers:  v,
	}
}

// Inventory returns the inventory of the crafter.
func (c Crafter) Inventory(*world.Tx, cube.Pos) *inventory.Inventory {
	return c.inventory
}

// WithName returns the crafter after applying a specific name to the block.
func (c Crafter) WithName(a ...any) world.Item {
	c.CustomName = strings.TrimSuffix(fmt.Sprintln(a...), "\n")
	return c
}

// AddViewer adds a viewer to the crafter, so that it is updated whenever the inventory of the crafter changes.
func (c Crafter) AddViewer(v ContainerViewer, _ *world.Tx, _ cube.Pos) {
	c.viewerMu.Lock()
	defer c.viewerMu.Unlock()
	c.viewers[v] = struct{}{}
}

// RemoveViewer removes a viewer from the crafter, so slot updates are no longer sent to it.
func (c Crafter) RemoveViewer(v ContainerViewer, _ *world.Tx, _ cube.Pos) {
	c.viewerMu.Lock()
	defer c.viewerMu.Unlock()
	delete(c.viewers, v)
}

// SlotEnabled returns true if the slot passed is enabled for crafting.
func (c Crafter) SlotEnabled(slot int) bool {
	if slot < 0 || slot >= 9 {
		return false
	}
	return c.DisabledSlots&(1<<slot) == 0
}

// SetSlotEnabled returns the crafter with the slot passed enabled or disabled.
func (c Crafter) SetSlotEnabled(slot int, enabled bool) Crafter {
	if slot < 0 || slot >= 9 {
		return c
	}
	if enabled {
		c.DisabledSlots &^= 1 << slot
	} else {
		c.DisabledSlots |= 1 << slot
	}
	return c
}

// Activate opens the crafter inventory.
func (c Crafter) Activate(pos cube.Pos, _ cube.Face, tx *world.Tx, u item.User, _ *item.UseContext) bool {
	if opener, ok := u.(ContainerOpener); ok {
		opener.OpenBlockContainer(pos, tx)
		return true
	}
	return false
}

// UseOnBlock places a crafter facing the user.
func (c Crafter) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) (used bool) {
	pos, _, used = firstReplaceable(tx, pos, face, c)
	if !used {
		return false
	}
	c = NewCrafter()
	c.Orientation = crafterOrientationFromFace(calculateFace(user, pos), user.Rotation().Direction().Opposite().Face())

	place(tx, pos, c, user, ctx)
	return placed(ctx)
}

// BreakInfo ...
func (c Crafter) BreakInfo() BreakInfo {
	return newBreakInfo(3.5, pickaxeHarvestable, pickaxeEffective, oneOf(Crafter{})).withBlastResistance(17.5).withBreakHandler(func(pos cube.Pos, tx *world.Tx, _ item.User) {
		for _, i := range c.Inventory(tx, pos).Clear() {
			dropItem(tx, i, pos.Vec3Centre())
		}
	})
}

// RedstonePowerUpdate returns a crafter with its triggered state matching the redstone power supplied.
func (c Crafter) RedstonePowerUpdate(_ cube.Pos, _ *world.Tx, power int) (world.Block, bool) {
	triggered := power > 0
	if c.Triggered == triggered {
		return c, false
	}
	c.Triggered = triggered
	if !triggered {
		c.Crafting = false
	}
	return c, true
}

// RedstonePowerAction crafts one recipe on a redstone rising edge.
func (c Crafter) RedstonePowerAction(pos cube.Pos, tx *world.Tx, oldPower, newPower int) bool {
	if oldPower > 0 || newPower == 0 {
		return false
	}
	return c.craft(pos, tx)
}

func (c Crafter) craft(pos cube.Pos, tx *world.Tx) bool {
	match, ok := c.matchRecipe()
	if !ok || len(match.output) == 0 {
		return false
	}
	facing := c.Orientation.Facing()
	if tx != nil {
		if _, ok := tx.BlockLoaded(pos.Side(facing)); !ok {
			return false
		}
	}
	for _, consume := range match.consume {
		stack, err := c.inventory.Item(consume.slot)
		if err != nil || stack.Empty() || stack.Count() < consume.count {
			return false
		}
	}
	for _, consume := range match.consume {
		stack, _ := c.inventory.Item(consume.slot)
		_ = c.inventory.SetItem(consume.slot, stack.Grow(-consume.count))
	}
	if tx != nil {
		for _, output := range match.output {
			if !output.Empty() {
				spawnRedstoneDispenserItem(tx, output, pos, facing)
			}
		}
	}
	return true
}

func (c Crafter) matchRecipe() (crafterRecipeMatch, bool) {
	if c.inventory == nil {
		return crafterRecipeMatch{}, false
	}
	slots := c.inventory.Slots()
	var (
		best     crafterRecipeMatch
		bestPrio uint32
		found    bool
	)
	for _, craft := range recipe.Recipes() {
		if craft.Block() != "crafting_table" {
			continue
		}
		var (
			match crafterRecipeMatch
			ok    bool
		)
		switch craft := craft.(type) {
		case recipe.Shaped:
			match, ok = crafterMatchShapedRecipe(slots, c.DisabledSlots, craft)
		case recipe.Shapeless:
			match, ok = crafterMatchShapelessRecipe(slots, c.DisabledSlots, craft)
		}
		if !ok {
			continue
		}
		if !found || craft.Priority() < bestPrio {
			best, bestPrio, found = match, craft.Priority(), true
		}
	}
	if found {
		return best, true
	}
	for _, craft := range recipe.DynamicRecipes() {
		if craft.Block() != "crafting_table" {
			continue
		}
		if match, ok := crafterMatchDynamicRecipe(slots, c.DisabledSlots, craft); ok {
			return match, true
		}
	}
	return crafterRecipeMatch{}, false
}

// RedstoneComparatorOutput returns the analog output for the crafter's contents.
func (c Crafter) RedstoneComparatorOutput(cube.Pos, *world.Tx, cube.Face) int {
	return redstoneComparatorOutputFromInventory(c.inventory)
}

// EncodeItem ...
func (Crafter) EncodeItem() (name string, meta int16) {
	return "minecraft:crafter", 0
}

// EncodeBlock ...
func (c Crafter) EncodeBlock() (string, map[string]any) {
	return "minecraft:crafter", map[string]any{
		"crafting":      boolByte(c.Crafting),
		"orientation":   c.Orientation.String(),
		"triggered_bit": boolByte(c.Triggered),
	}
}

// EncodeNBT ...
func (c Crafter) EncodeNBT() map[string]any {
	if c.inventory == nil {
		orientation, triggered, crafting, disabledSlots, customName := c.Orientation, c.Triggered, c.Crafting, c.DisabledSlots, c.CustomName
		c = NewCrafter()
		c.Orientation, c.Triggered, c.Crafting, c.DisabledSlots, c.CustomName = orientation, triggered, crafting, disabledSlots, customName
	}
	m := map[string]any{
		"DisabledSlots": int32(c.DisabledSlots & 0x1ff),
		"Items":         nbtconv.InvToNBT(c.inventory),
		"id":            "Crafter",
	}
	if c.CustomName != "" {
		m["CustomName"] = c.CustomName
	}
	return m
}

// DecodeNBT ...
func (c Crafter) DecodeNBT(data map[string]any) any {
	orientation, triggered, crafting := c.Orientation, c.Triggered, c.Crafting
	c = NewCrafter()
	c.Orientation, c.Triggered, c.Crafting = orientation, triggered, crafting
	c.CustomName = nbtconv.String(data, "CustomName")
	c.DisabledSlots = uint16(nbtconv.Int32(data, "DisabledSlots")) & 0x1ff
	if disabled, ok := data["disabled_slots"].(int32); ok {
		c.DisabledSlots = uint16(disabled) & 0x1ff
	}
	nbtconv.InvFromNBT(c.inventory, crafterNBTItems(data))
	return c
}

type crafterRecipeMatch struct {
	output  []item.Stack
	consume []crafterRecipeConsume
}

type crafterRecipeConsume struct {
	slot, count int
}

func crafterMatchShapedRecipe(slots []item.Stack, disabledSlots uint16, craft recipe.Shaped) (crafterRecipeMatch, bool) {
	shape := craft.Shape()
	width, height := shape.Width(), shape.Height()
	input := craft.Input()
	if width <= 0 || height <= 0 || width > 3 || height > 3 || len(input) != width*height {
		return crafterRecipeMatch{}, false
	}
	for yOffset := 0; yOffset <= 3-height; yOffset++ {
		for xOffset := 0; xOffset <= 3-width; xOffset++ {
			var consume []crafterRecipeConsume
			matches := true
			for y := 0; y < 3 && matches; y++ {
				for x := 0; x < 3; x++ {
					slot := y*3 + x
					stack := crafterSlotStack(slots, disabledSlots, slot)
					if x < xOffset || x >= xOffset+width || y < yOffset || y >= yOffset+height {
						if !stack.Empty() {
							matches = false
							break
						}
						continue
					}
					expected := input[(y-yOffset)*width+(x-xOffset)]
					if expected.Empty() {
						if !stack.Empty() {
							matches = false
							break
						}
						continue
					}
					if stack.Empty() || stack.Count() < expected.Count() || !crafterRecipeItemMatches(stack, expected) {
						matches = false
						break
					}
					consume = append(consume, crafterRecipeConsume{slot: slot, count: expected.Count()})
				}
			}
			if matches {
				return crafterRecipeMatch{output: cloneCrafterRecipeOutput(craft.Output()), consume: consume}, true
			}
		}
	}
	return crafterRecipeMatch{}, false
}

func crafterMatchShapelessRecipe(slots []item.Stack, disabledSlots uint16, craft recipe.Shapeless) (crafterRecipeMatch, bool) {
	input := make([]recipe.Item, 0, len(craft.Input()))
	for _, expected := range craft.Input() {
		if !expected.Empty() {
			input = append(input, expected)
		}
	}
	used := [9]bool{}
	consume := make([]crafterRecipeConsume, len(input))
	var match func(int) bool
	match = func(inputIndex int) bool {
		if inputIndex == len(input) {
			for slot := 0; slot < 9; slot++ {
				if !used[slot] && !crafterSlotStack(slots, disabledSlots, slot).Empty() {
					return false
				}
			}
			return true
		}
		expected := input[inputIndex]
		for slot := 0; slot < 9; slot++ {
			if used[slot] {
				continue
			}
			stack := crafterSlotStack(slots, disabledSlots, slot)
			if stack.Empty() || stack.Count() < expected.Count() || !crafterRecipeItemMatches(stack, expected) {
				continue
			}
			used[slot] = true
			consume[inputIndex] = crafterRecipeConsume{slot: slot, count: expected.Count()}
			if match(inputIndex + 1) {
				return true
			}
			used[slot] = false
		}
		return false
	}
	if !match(0) {
		return crafterRecipeMatch{}, false
	}
	return crafterRecipeMatch{output: cloneCrafterRecipeOutput(craft.Output()), consume: consume}, true
}

func crafterMatchDynamicRecipe(slots []item.Stack, disabledSlots uint16, craft recipe.DynamicRecipe) (crafterRecipeMatch, bool) {
	input := make([]recipe.Item, 9)
	consume := make([]crafterRecipeConsume, 0, 9)
	for slot := 0; slot < 9; slot++ {
		stack := crafterSlotStack(slots, disabledSlots, slot)
		if stack.Empty() {
			input[slot] = item.Stack{}
			continue
		}
		input[slot] = stack
		consume = append(consume, crafterRecipeConsume{slot: slot, count: 1})
	}
	output, ok := craft.Match(input)
	if !ok {
		return crafterRecipeMatch{}, false
	}
	return crafterRecipeMatch{output: cloneCrafterRecipeOutput(output), consume: consume}, true
}

func crafterRecipeItemMatches(has item.Stack, expected recipe.Item) bool {
	switch expected := expected.(type) {
	case item.Stack:
		if _, variants := expected.Value("variants"); variants {
			nameOne, _ := has.Item().EncodeItem()
			nameTwo, _ := expected.Item().EncodeItem()
			return nameOne == nameTwo
		}
		return has.Comparable(expected)
	case recipe.ItemTag:
		name, _ := has.Item().EncodeItem()
		return expected.Contains(name)
	default:
		return false
	}
}

func crafterSlotStack(slots []item.Stack, disabledSlots uint16, slot int) item.Stack {
	if slot < 0 || slot >= len(slots) || disabledSlots&(1<<slot) != 0 {
		return item.Stack{}
	}
	return slots[slot]
}

func cloneCrafterRecipeOutput(output []item.Stack) []item.Stack {
	return append([]item.Stack(nil), output...)
}

func crafterNBTItems(data map[string]any) []any {
	items := nbtconv.Slice(data, "Items")
	if items != nil {
		return items
	}
	if mapped, ok := data["Items"].([]map[string]any); ok {
		items = make([]any, 0, len(mapped))
		for _, itemData := range mapped {
			items = append(items, itemData)
		}
	}
	return items
}

// CrafterOrientation is the orientation of a crafter block.
type CrafterOrientation uint8

const (
	CrafterOrientationDownEast CrafterOrientation = iota
	CrafterOrientationDownNorth
	CrafterOrientationDownSouth
	CrafterOrientationDownWest
	CrafterOrientationUpEast
	CrafterOrientationUpNorth
	CrafterOrientationUpSouth
	CrafterOrientationUpWest
	CrafterOrientationWestUp
	CrafterOrientationEastUp
	CrafterOrientationNorthUp
	CrafterOrientationSouthUp
)

// String returns the Bedrock block state name for the orientation.
func (o CrafterOrientation) String() string {
	switch o {
	case CrafterOrientationDownEast:
		return "down_east"
	case CrafterOrientationDownNorth:
		return "down_north"
	case CrafterOrientationDownSouth:
		return "down_south"
	case CrafterOrientationDownWest:
		return "down_west"
	case CrafterOrientationUpEast:
		return "up_east"
	case CrafterOrientationUpNorth:
		return "up_north"
	case CrafterOrientationUpSouth:
		return "up_south"
	case CrafterOrientationUpWest:
		return "up_west"
	case CrafterOrientationWestUp:
		return "west_up"
	case CrafterOrientationEastUp:
		return "east_up"
	case CrafterOrientationNorthUp:
		return "north_up"
	case CrafterOrientationSouthUp:
		return "south_up"
	}
	panic("invalid crafter orientation")
}

// Facing returns the output face of the crafter orientation.
func (o CrafterOrientation) Facing() cube.Face {
	switch o {
	case CrafterOrientationDownEast, CrafterOrientationDownNorth, CrafterOrientationDownSouth, CrafterOrientationDownWest:
		return cube.FaceDown
	case CrafterOrientationUpEast, CrafterOrientationUpNorth, CrafterOrientationUpSouth, CrafterOrientationUpWest:
		return cube.FaceUp
	case CrafterOrientationWestUp:
		return cube.FaceWest
	case CrafterOrientationEastUp:
		return cube.FaceEast
	case CrafterOrientationNorthUp:
		return cube.FaceNorth
	case CrafterOrientationSouthUp:
		return cube.FaceSouth
	}
	panic("invalid crafter orientation")
}

// Uint8 returns the orientation as a compact integer.
func (o CrafterOrientation) Uint8() uint8 {
	return uint8(o)
}

func crafterOrientationFromFace(face, top cube.Face) CrafterOrientation {
	switch face {
	case cube.FaceDown:
		switch top {
		case cube.FaceNorth:
			return CrafterOrientationDownNorth
		case cube.FaceSouth:
			return CrafterOrientationDownSouth
		case cube.FaceWest:
			return CrafterOrientationDownWest
		default:
			return CrafterOrientationDownEast
		}
	case cube.FaceUp:
		switch top {
		case cube.FaceNorth:
			return CrafterOrientationUpNorth
		case cube.FaceSouth:
			return CrafterOrientationUpSouth
		case cube.FaceWest:
			return CrafterOrientationUpWest
		default:
			return CrafterOrientationUpEast
		}
	case cube.FaceWest:
		return CrafterOrientationWestUp
	case cube.FaceEast:
		return CrafterOrientationEastUp
	case cube.FaceSouth:
		return CrafterOrientationSouthUp
	default:
		return CrafterOrientationNorthUp
	}
}

func allCrafters() (crafters []world.Block) {
	for _, orientation := range crafterOrientations() {
		for _, crafting := range []bool{false, true} {
			for _, triggered := range []bool{false, true} {
				crafters = append(crafters, Crafter{Orientation: orientation, Crafting: crafting, Triggered: triggered})
			}
		}
	}
	return
}

func crafterOrientations() []CrafterOrientation {
	return []CrafterOrientation{
		CrafterOrientationDownEast,
		CrafterOrientationDownNorth,
		CrafterOrientationDownSouth,
		CrafterOrientationDownWest,
		CrafterOrientationUpEast,
		CrafterOrientationUpNorth,
		CrafterOrientationUpSouth,
		CrafterOrientationUpWest,
		CrafterOrientationWestUp,
		CrafterOrientationEastUp,
		CrafterOrientationNorthUp,
		CrafterOrientationSouthUp,
	}
}

func crafterStatesSupported() bool {
	for _, orientation := range crafterOrientations() {
		for _, crafting := range []bool{false, true} {
			for _, triggered := range []bool{false, true} {
				if _, ok := world.BlockByName("minecraft:crafter", map[string]any{
					"crafting":      boolByte(crafting),
					"orientation":   orientation.String(),
					"triggered_bit": boolByte(triggered),
				}); !ok {
					return false
				}
			}
		}
	}
	return true
}
