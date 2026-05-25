package chunk

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
)

var (
	benchmarkEncodedData SerialisedData
	benchmarkEncodedBlob []byte
)

type benchmarkBlockRegistry struct{}

func (benchmarkBlockRegistry) BlockCount() int { return 64 }

func (benchmarkBlockRegistry) AirRuntimeID() uint32 { return 0 }

func (benchmarkBlockRegistry) RuntimeIDToState(runtimeID uint32) (string, map[string]any, bool) {
	return fmt.Sprintf("benchmark:block_%d", runtimeID), nil, true
}

func (benchmarkBlockRegistry) StateToRuntimeID(string, map[string]any) (uint32, bool) {
	return 0, true
}

func (benchmarkBlockRegistry) FilteringBlock(rid uint32) uint8 {
	if rid == 0 {
		return 0
	}
	return 15
}

func (benchmarkBlockRegistry) LightBlock(uint32) uint8 { return 0 }

func (benchmarkBlockRegistry) RandomTickBlock(uint32) bool { return false }

func (benchmarkBlockRegistry) NBTBlock(uint32) bool { return false }

func (benchmarkBlockRegistry) LiquidDisplacingBlock(uint32) bool { return false }

func (benchmarkBlockRegistry) LiquidBlock(uint32) bool { return false }

func (benchmarkBlockRegistry) HashToRuntimeID(hash uint32) (uint32, bool) { return hash, true }

func benchmarkChunk() *Chunk {
	c := New(benchmarkBlockRegistry{}, cube.Range{-64, 319})
	for y := int16(-64); y <= 319; y++ {
		for x := uint8(0); x < 16; x++ {
			for z := uint8(0); z < 16; z++ {
				rid := uint32(1 + (int(y+64)+int(x)+int(z))%31)
				c.SetBlock(x, y, z, 0, rid)
				c.SetBiome(x, y, z, uint32(1+(int(y+64)>>4)%8))
			}
		}
	}
	return c
}

func TestNetworkEncodeInvalidatesAfterBlockMutation(t *testing.T) {
	c := New(benchmarkBlockRegistry{}, cube.Range{0, 15})
	before := append([]byte(nil), NetworkEncodeSubChunk(c, 0)...)

	c.SetBlock(0, 0, 0, 0, 1)
	after := NetworkEncodeSubChunk(c, 0)
	if bytes.Equal(before, after) {
		t.Fatal("network encoded sub-chunk was not refreshed after block mutation")
	}
}

func TestNetworkEncodeInvalidatesAfterBiomeMutation(t *testing.T) {
	c := New(benchmarkBlockRegistry{}, cube.Range{0, 15})
	before := append([]byte(nil), NetworkEncodeBiomes(c)...)

	c.SetBiome(0, 0, 0, 1)
	after := NetworkEncodeBiomes(c)
	if bytes.Equal(before, after) {
		t.Fatal("network encoded biomes were not refreshed after biome mutation")
	}
}

func BenchmarkNetworkEncodeSharedChunk(b *testing.B) {
	c := benchmarkChunk()
	for _, viewers := range []int{1, 16, 64} {
		b.Run(fmt.Sprintf("viewers_%d", viewers), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				for range viewers {
					benchmarkEncodedData = NetworkEncode(c)
				}
			}
		})
	}
}

func BenchmarkNetworkEncodeSubChunkSharedChunk(b *testing.B) {
	c := benchmarkChunk()
	for _, viewers := range []int{1, 16, 64} {
		b.Run(fmt.Sprintf("viewers_%d", viewers), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				for viewer := 0; viewer < viewers; viewer++ {
					benchmarkEncodedBlob = NetworkEncodeSubChunk(c, viewer%len(c.sub))
				}
			}
		})
	}
}

func BenchmarkNetworkEncodeBiomesSharedChunk(b *testing.B) {
	c := benchmarkChunk()
	for _, viewers := range []int{1, 16, 64} {
		b.Run(fmt.Sprintf("viewers_%d", viewers), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				for range viewers {
					benchmarkEncodedBlob = NetworkEncodeBiomes(c)
				}
			}
		})
	}
}
