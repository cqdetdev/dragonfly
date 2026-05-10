package block

import (
	"fmt"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/sound"
	"github.com/go-gl/mathgl/mgl64"
)

func TestPistonPushPlanAllowsTwelveBlocks(t *testing.T) {
	pos := cube.Pos{0, 64, 0}
	blocks := map[cube.Pos]world.Block{}
	for i := 1; i <= pistonPushLimit; i++ {
		blocks[cube.Pos{i, 64, 0}] = Stone{}
	}
	blocks[cube.Pos{pistonPushLimit + 1, 64, 0}] = Air{}

	moves, ok := pistonPushPlan(pos, cube.FaceEast, cube.Range{-64, 320}, mapBlockLookup(blocks))
	if !ok {
		t.Fatal("piston push plan rejected 12 movable blocks")
	}
	if len(moves) != pistonPushLimit {
		t.Fatalf("piston push plan returned %d moves, want %d", len(moves), pistonPushLimit)
	}
	if got, want := moves[0].from, (cube.Pos{pistonPushLimit, 64, 0}); got != want {
		t.Fatalf("first piston move source = %v, want %v", got, want)
	}
	if got, want := moves[0].to, (cube.Pos{pistonPushLimit + 1, 64, 0}); got != want {
		t.Fatalf("first piston move target = %v, want %v", got, want)
	}
	if got, want := moves[len(moves)-1].from, (cube.Pos{1, 64, 0}); got != want {
		t.Fatalf("last piston move source = %v, want %v", got, want)
	}
	if got, want := moves[len(moves)-1].to, (cube.Pos{2, 64, 0}); got != want {
		t.Fatalf("last piston move target = %v, want %v", got, want)
	}
}

func TestPistonPushPlanRejectsThirteenBlocks(t *testing.T) {
	pos := cube.Pos{0, 64, 0}
	blocks := map[cube.Pos]world.Block{}
	for i := 1; i <= pistonPushLimit+1; i++ {
		blocks[cube.Pos{i, 64, 0}] = Stone{}
	}
	blocks[cube.Pos{pistonPushLimit + 2, 64, 0}] = Air{}

	if _, ok := pistonPushPlan(pos, cube.FaceEast, cube.Range{-64, 320}, mapBlockLookup(blocks)); ok {
		t.Fatal("piston push plan accepted 13 movable blocks")
	}
}

func TestPistonPushPlanRejectsImmovableBlock(t *testing.T) {
	pos := cube.Pos{0, 64, 0}
	blocks := map[cube.Pos]world.Block{
		{1, 64, 0}: Bedrock{},
		{2, 64, 0}: Air{},
	}
	if _, ok := pistonPushPlan(pos, cube.FaceEast, cube.Range{-64, 320}, mapBlockLookup(blocks)); ok {
		t.Fatal("piston push plan accepted bedrock")
	}
}

func TestPistonPushPlanRejectsAttachedRedstone(t *testing.T) {
	pos := cube.Pos{0, 64, 0}
	blocks := map[cube.Pos]world.Block{
		{1, 64, 0}: Lever{},
		{2, 64, 0}: Air{},
	}
	if _, ok := pistonPushPlan(pos, cube.FaceEast, cube.Range{-64, 320}, mapBlockLookup(blocks)); ok {
		t.Fatal("piston push plan accepted lever")
	}
}

func TestStickyPistonPullPlanPullsOneBlock(t *testing.T) {
	pos := cube.Pos{0, 64, 0}
	blocks := map[cube.Pos]world.Block{
		{1, 64, 0}: Air{},
		{2, 64, 0}: Stone{},
	}
	move, ok := pistonPullPlan(pos, cube.FaceEast, cube.Range{-64, 320}, mapBlockLookup(blocks))
	if !ok {
		t.Fatal("sticky piston pull plan rejected one movable block")
	}
	if got, want := move.from, (cube.Pos{2, 64, 0}); got != want {
		t.Fatalf("sticky piston pull source = %v, want %v", got, want)
	}
	if got, want := move.to, (cube.Pos{1, 64, 0}); got != want {
		t.Fatalf("sticky piston pull target = %v, want %v", got, want)
	}
	if _, ok := move.block.(Stone); !ok {
		t.Fatalf("sticky piston pull block = %#v, want Stone", move.block)
	}
}

func TestPistonEncodeAndStateCounts(t *testing.T) {
	name, props := (Piston{Facing: cube.FaceEast, Extended: true}).EncodeBlock()
	if name != "minecraft:piston" {
		t.Fatalf("piston block name = %q, want minecraft:piston", name)
	}
	if facing := props["facing_direction"]; facing != int32(cube.FaceWest) {
		t.Fatalf("piston facing_direction = %v, want %d", facing, cube.FaceWest)
	}
	if _, ok := props["extended"]; ok {
		t.Fatal("piston EncodeBlock exposed NBT-only extended state")
	}
	if _, ok := world.BlockByName(name, props); !ok {
		t.Fatalf("BlockByName(%s, %#v) was not found", name, props)
	}
	_, props = (Piston{Facing: cube.FaceUp}).EncodeBlock()
	if facing := props["facing_direction"]; facing != int32(cube.FaceUp) {
		t.Fatalf("up piston facing_direction = %v, want %d", facing, cube.FaceUp)
	}
	_, props = (Piston{Facing: cube.FaceDown}).EncodeBlock()
	if facing := props["facing_direction"]; facing != int32(cube.FaceDown) {
		t.Fatalf("down piston facing_direction = %v, want %d", facing, cube.FaceDown)
	}

	name, props = (StickyPiston{Facing: cube.FaceNorth, Extended: true}).EncodeBlock()
	if name != "minecraft:sticky_piston" {
		t.Fatalf("sticky piston block name = %q, want minecraft:sticky_piston", name)
	}
	if facing := props["facing_direction"]; facing != int32(cube.FaceSouth) {
		t.Fatalf("sticky piston facing_direction = %v, want %d", facing, cube.FaceSouth)
	}
	if _, ok := world.BlockByName(name, props); !ok {
		t.Fatalf("BlockByName(%s, %#v) was not found", name, props)
	}
	if len(allPistons()) != len(cube.Faces()) {
		t.Fatalf("allPistons returned %d states, want %d", len(allPistons()), len(cube.Faces()))
	}
	if len(allStickyPistons()) != len(cube.Faces()) {
		t.Fatalf("allStickyPistons returned %d states, want %d", len(allStickyPistons()), len(cube.Faces()))
	}

	name, props = (PistonArm{Facing: cube.FaceEast}).EncodeBlock()
	if name != "minecraft:piston_arm_collision" {
		t.Fatalf("piston arm block name = %q, want minecraft:piston_arm_collision", name)
	}
	if facing := props["facing_direction"]; facing != int32(cube.FaceWest) {
		t.Fatalf("piston arm facing_direction = %v, want %d", facing, cube.FaceWest)
	}
	if _, ok := world.BlockByName(name, props); !ok {
		t.Fatalf("BlockByName(%s, %#v) was not found", name, props)
	}
	name, props = (PistonArm{Facing: cube.FaceUp}).EncodeBlock()
	if name != "minecraft:piston_arm_collision" {
		t.Fatalf("up piston arm block name = %q, want minecraft:piston_arm_collision", name)
	}
	if facing := props["facing_direction"]; facing != int32(cube.FaceUp) {
		t.Fatalf("up piston arm facing_direction = %v, want %d", facing, cube.FaceUp)
	}
	name, props = (PistonArm{Facing: cube.FaceEast, Sticky: true}).EncodeBlock()
	if name != "minecraft:sticky_piston_arm_collision" {
		t.Fatalf("sticky piston arm block name = %q, want minecraft:sticky_piston_arm_collision", name)
	}
	if facing := props["facing_direction"]; facing != int32(cube.FaceWest) {
		t.Fatalf("sticky piston arm facing_direction = %v, want %d", facing, cube.FaceWest)
	}
	if _, ok := world.BlockByName(name, props); !ok {
		t.Fatalf("BlockByName(%s, %#v) was not found", name, props)
	}
}

func TestPistonNBTState(t *testing.T) {
	if extended := (Piston{}).DecodeNBT(map[string]any{"extended": uint8(1)}).(Piston).Extended; !extended {
		t.Fatal("piston NBT did not decode extended state")
	}
	if extended := (Piston{}).DecodeNBT(map[string]any{"State": pistonStateExtended}).(Piston).Extended; !extended {
		t.Fatal("piston NBT did not decode Bedrock extended state")
	}
	nbt := (StickyPiston{Facing: cube.FaceEast, Extended: true}).EncodeNBT()
	if got := nbt["extended"]; got != uint8(1) {
		t.Fatalf("sticky piston encoded extended = %v, want 1", got)
	}
	if got := nbt["id"]; got != "PistonArm" {
		t.Fatalf("sticky piston block actor id = %v, want PistonArm", got)
	}
	if got := nbt["facing"]; got != int32(cube.FaceWest) {
		t.Fatalf("sticky piston block actor facing = %v, want %d", got, cube.FaceWest)
	}
	nbt = (StickyPiston{Facing: cube.FaceUp, Extended: true}).EncodeNBT()
	if got := nbt["facing"]; got != int32(cube.FaceUp) {
		t.Fatalf("up sticky piston block actor facing = %v, want %d", got, cube.FaceUp)
	}
	if got := nbt["State"]; got != pistonStateExtended {
		t.Fatalf("sticky piston block actor state = %v, want %d", got, pistonStateExtended)
	}
}

func TestPistonPlacesAndRemovesArm(t *testing.T) {
	w := world.New()
	defer func() {
		_ = w.Close()
	}()
	h := &pistonSoundHandler{}
	w.Handle(h)

	var err error
	<-w.Exec(func(tx *world.Tx) {
		pos := cube.Pos{0, 1, 0}
		p := Piston{Facing: cube.FaceEast}
		tx.SetBlock(pos, p, nil)
		p.ScheduledTick(pos, tx, nil)
		if _, ok := tx.Block(pos.Side(cube.FaceEast)).(PistonArm); ok {
			err = fmt.Errorf("unpowered piston placed arm")
			return
		}

		tx.SetBlock(pos.Side(cube.FaceWest), RedstoneBlock{}, nil)
		p.ScheduledTick(pos, tx, nil)
		arm, ok := tx.Block(pos.Side(cube.FaceEast)).(PistonArm)
		if !ok {
			err = fmt.Errorf("powered piston did not place arm")
			return
		}
		if arm.Facing != cube.FaceEast || arm.Sticky {
			err = fmt.Errorf("piston arm = %#v, want east non-sticky", arm)
			return
		}
		extended := tx.Block(pos).(Piston)
		if !extended.Extended {
			err = fmt.Errorf("piston did not store extended state")
			return
		}

		tx.SetBlock(pos.Side(cube.FaceWest), nil, nil)
		extended.ScheduledTick(pos, tx, nil)
		if _, ok := tx.Block(pos.Side(cube.FaceEast)).(PistonArm); ok {
			err = fmt.Errorf("retracted piston left arm behind")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(h.sounds) != 2 {
		t.Fatalf("piston played %d sounds, want 2", len(h.sounds))
	}
	if _, ok := h.sounds[0].(sound.PistonOut); !ok {
		t.Fatalf("piston extend sound = %T, want sound.PistonOut", h.sounds[0])
	}
	if _, ok := h.sounds[1].(sound.PistonIn); !ok {
		t.Fatalf("piston retract sound = %T, want sound.PistonIn", h.sounds[1])
	}
}

type pistonSoundHandler struct {
	world.NopHandler
	sounds []world.Sound
}

func (h *pistonSoundHandler) HandleSound(_ *world.Context, s world.Sound, _ mgl64.Vec3) {
	h.sounds = append(h.sounds, s)
}

func mapBlockLookup(blocks map[cube.Pos]world.Block) pistonBlockLookup {
	return func(pos cube.Pos) (world.Block, bool) {
		b, ok := blocks[pos]
		return b, ok
	}
}
