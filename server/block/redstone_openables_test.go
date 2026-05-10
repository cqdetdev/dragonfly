package block

import (
	"fmt"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

func TestRedstoneOpenableConsumers(t *testing.T) {
	if after, changed := (WoodTrapdoor{}).RedstonePowerUpdate(cube.Pos{}, nil, 15); !changed || !after.(WoodTrapdoor).Open {
		t.Fatalf("powered WoodTrapdoor update = %#v, %v; want open change", after, changed)
	}
	if after, changed := (WoodFenceGate{}).RedstonePowerUpdate(cube.Pos{}, nil, 15); !changed || !after.(WoodFenceGate).Open {
		t.Fatalf("powered WoodFenceGate update = %#v, %v; want open change", after, changed)
	}
	if after, changed := (WoodDoor{}).RedstonePowerUpdate(cube.Pos{}, nil, 15); !changed || !after.(WoodDoor).Open {
		t.Fatalf("powered WoodDoor update = %#v, %v; want open change", after, changed)
	}
	if after, changed := (CopperTrapdoor{}).RedstonePowerUpdate(cube.Pos{}, nil, 15); !changed || !after.(CopperTrapdoor).Open {
		t.Fatalf("powered CopperTrapdoor update = %#v, %v; want open change", after, changed)
	}
	if after, changed := (CopperDoor{}).RedstonePowerUpdate(cube.Pos{}, nil, 15); !changed || !after.(CopperDoor).Open {
		t.Fatalf("powered CopperDoor update = %#v, %v; want open change", after, changed)
	}
	if after, changed := (IronDoor{}).RedstonePowerUpdate(cube.Pos{}, nil, 15); !changed || !after.(IronDoor).Open {
		t.Fatalf("powered IronDoor update = %#v, %v; want open change", after, changed)
	}
	if after, changed := (IronTrapdoor{}).RedstonePowerUpdate(cube.Pos{}, nil, 15); !changed || !after.(IronTrapdoor).Open {
		t.Fatalf("powered IronTrapdoor update = %#v, %v; want open change", after, changed)
	}
	if (IronDoor{}).Activate(cube.Pos{}, cube.FaceUp, nil, nil, nil) {
		t.Fatal("IronDoor activated without redstone")
	}
	if (IronTrapdoor{}).Activate(cube.Pos{}, cube.FaceUp, nil, nil, nil) {
		t.Fatal("IronTrapdoor activated without redstone")
	}
}

func TestRedstoneDoorUpdateSyncsOtherHalf(t *testing.T) {
	w := world.New()
	defer func() {
		_ = w.Close()
	}()

	var err error
	<-w.Exec(func(tx *world.Tx) {
		pos := cube.Pos{0, 1, 0}
		topPos := pos.Side(cube.FaceUp)

		tx.SetBlock(pos, WoodDoor{}, nil)
		top := WoodDoor{Top: true}
		after, changed := top.RedstonePowerUpdate(topPos, tx, 15)
		if !changed {
			err = fmt.Errorf("wood door top half did not change")
			return
		}
		tx.SetBlock(topPos, after, nil)
		top.RedstonePowerPostUpdate(topPos, tx, top, after, 0, 15)
		if bottom := tx.Block(pos).(WoodDoor); !bottom.Open {
			err = fmt.Errorf("wood door bottom half did not sync open")
			return
		}

		tx.SetBlock(pos, IronDoor{}, nil)
		ironTop := IronDoor{Top: true}
		after, changed = ironTop.RedstonePowerUpdate(topPos, tx, 15)
		if !changed {
			err = fmt.Errorf("iron door top half did not change")
			return
		}
		tx.SetBlock(topPos, after, nil)
		ironTop.RedstonePowerPostUpdate(topPos, tx, ironTop, after, 0, 15)
		if bottom := tx.Block(pos).(IronDoor); !bottom.Open {
			err = fmt.Errorf("iron door bottom half did not sync open")
			return
		}

		tx.SetBlock(pos, CopperDoor{}, nil)
		copperTop := CopperDoor{Top: true}
		after, changed = copperTop.RedstonePowerUpdate(topPos, tx, 15)
		if !changed {
			err = fmt.Errorf("copper door top half did not change")
			return
		}
		tx.SetBlock(topPos, after, nil)
		copperTop.RedstonePowerPostUpdate(topPos, tx, copperTop, after, 0, 15)
		if bottom := tx.Block(pos).(CopperDoor); !bottom.Open {
			err = fmt.Errorf("copper door bottom half did not sync open")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRedstoneDoorHalfUsesPairedPower(t *testing.T) {
	w := world.New()
	defer func() {
		_ = w.Close()
	}()

	var err error
	<-w.Exec(func(tx *world.Tx) {
		pos := cube.Pos{0, 1, 0}
		topPos := pos.Side(cube.FaceUp)
		tx.SetBlock(pos, WoodDoor{Open: true}, nil)
		tx.SetBlock(topPos, WoodDoor{Top: true, Open: true}, nil)
		tx.SetBlock(pos.Side(cube.FaceEast), RedstoneBlock{}, nil)

		after, changed := (WoodDoor{Top: true, Open: true}).RedstonePowerUpdate(topPos, tx, 0)
		if changed {
			err = fmt.Errorf("unpowered door half changed while paired half was powered: %#v", after)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestOpenablesIgnoreSamePowerRedstoneInvalidation(t *testing.T) {
	if after, changed := (WoodDoor{Open: true}).RedstonePowerTransitionUpdate(cube.Pos{}, nil, 0, 0); changed || !after.(WoodDoor).Open {
		t.Fatalf("wood door 0->0 transition = %#v, %v; want unchanged open", after, changed)
	}
	if after, changed := (WoodDoor{Open: true}).RedstonePowerTransitionUpdate(cube.Pos{}, nil, 15, 0); !changed || after.(WoodDoor).Open {
		t.Fatalf("wood door 15->0 transition = %#v, %v; want closed", after, changed)
	}
	if after, changed := (WoodTrapdoor{Open: true}).RedstonePowerTransitionUpdate(cube.Pos{}, nil, 0, 0); changed || !after.(WoodTrapdoor).Open {
		t.Fatalf("wood trapdoor 0->0 transition = %#v, %v; want unchanged open", after, changed)
	}
	if after, changed := (WoodFenceGate{Open: true}).RedstonePowerTransitionUpdate(cube.Pos{}, nil, 0, 0); changed || !after.(WoodFenceGate).Open {
		t.Fatalf("wood fence gate 0->0 transition = %#v, %v; want unchanged open", after, changed)
	}
	if after, changed := (IronDoor{}).RedstonePowerTransitionUpdate(cube.Pos{}, nil, 0, 15); !changed || !after.(IronDoor).Open {
		t.Fatalf("iron door 0->15 transition = %#v, %v; want open", after, changed)
	}
}
