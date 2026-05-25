package chunk

import (
	"bytes"
	"sync"
)

const (
	// SubChunkVersion is the current version of the written sub chunks, specifying the format they are
	// written on disk and over network.
	SubChunkVersion = 9
	// CurrentBlockVersion is the current version of blocks (states) of the game. This version is composed
	// of 4 bytes indicating a version, interpreted as a big endian int. The current version represents
	// 1.16.0.14 {1, 16, 0, 14}.
	CurrentBlockVersion int32 = 18040335
)

var (
	// pool is used to pool byte buffers used for encoding chunks.
	pool = sync.Pool{
		New: func() any {
			return bytes.NewBuffer(make([]byte, 0, 1024))
		},
	}
)

type (
	// SerialisedData holds the serialised data of a chunk. It consists of the chunk's block data itself, a height
	// map, the biomes and entities and block entities.
	SerialisedData struct {
		// sub holds the data of the serialised sub chunks in a chunk. Sub chunks that are empty or that otherwise
		// don't exist are represented as an empty slice (or technically, nil).
		SubChunks [][]byte
		// Biomes is the biome data of the chunk, which is composed of a biome storage for each sub-chunk.
		Biomes []byte
	}
	// blockEntry represents a block as found in a disk save of a world.
	blockEntry struct {
		Name    string         `nbt:"name"`
		State   map[string]any `nbt:"states"`
		Version int32          `nbt:"version"`
	}
	networkEncodeCache struct {
		valid     bool
		subChunks [][]byte
		biomes    []byte
	}
)

// Encode encodes Chunk to an intermediate representation SerialisedData. An Encoding may be passed to encode either for
// network or disk purposed, the most notable difference being that the network encoding generally uses varints and no
// NBT.
func Encode(c *Chunk, e Encoding) SerialisedData {
	d := SerialisedData{SubChunks: make([][]byte, len(c.sub))}
	for i := range c.sub {
		d.SubChunks[i] = encodeSubChunk(c, e, i)
	}
	d.Biomes = encodeBiomes(c, e)
	return d
}

// NetworkEncode encodes Chunk using the network encoding. The returned slices are cached and must not be mutated.
func NetworkEncode(c *Chunk) SerialisedData {
	cache := c.networkEncoding()
	for i := range c.sub {
		if cache.subChunks[i] == nil {
			cache.subChunks[i] = encodeSubChunk(c, NetworkEncoding, i)
		}
	}
	if cache.biomes == nil {
		cache.biomes = encodeBiomes(c, NetworkEncoding)
	}
	return SerialisedData{SubChunks: cache.subChunks, Biomes: cache.biomes}
}

// EncodeSubChunk encodes a sub-chunk from a chunk into bytes. An Encoding may be passed to encode either for network or
// disk purposed, the most notable difference being that the network encoding generally uses varints and no NBT.
func EncodeSubChunk(c *Chunk, e Encoding, ind int) []byte {
	return encodeSubChunk(c, e, ind)
}

func encodeSubChunk(c *Chunk, e Encoding, ind int) []byte {
	buf := pool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		pool.Put(buf)
	}()

	s := c.sub[ind]
	_, _ = buf.Write([]byte{SubChunkVersion, byte(len(s.storages)), uint8(ind + (c.r[0] >> 4))})
	for _, storage := range s.storages {
		encodePalettedStorage(buf, storage, nil, e, BlockPaletteEncoding{Blocks: c.br})
	}
	sub := make([]byte, buf.Len())
	_, _ = buf.Read(sub)
	return sub
}

// NetworkEncodeSubChunk encodes a sub-chunk from a chunk using the network encoding. The returned slice is cached and
// must not be mutated.
func NetworkEncodeSubChunk(c *Chunk, ind int) []byte {
	cache := c.networkEncoding()
	if cache.subChunks[ind] == nil {
		cache.subChunks[ind] = encodeSubChunk(c, NetworkEncoding, ind)
	}
	return cache.subChunks[ind]
}

// EncodeBiomes encodes the biomes of a chunk into bytes. An Encoding may be passed to encode either for network or
// disk purposed, the most notable difference being that the network encoding generally uses varints and no NBT.
func EncodeBiomes(c *Chunk, e Encoding) []byte {
	return encodeBiomes(c, e)
}

func encodeBiomes(c *Chunk, e Encoding) []byte {
	buf := pool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		pool.Put(buf)
	}()

	var previous *PalettedStorage
	for _, b := range c.biomes {
		encodePalettedStorage(buf, b, previous, e, BiomePaletteEncoding)
		previous = b
	}
	biomes := make([]byte, buf.Len())
	_, _ = buf.Read(biomes)
	return biomes
}

// NetworkEncodeBiomes encodes the biomes of a chunk using the network encoding. The returned slice is cached and must
// not be mutated.
func NetworkEncodeBiomes(c *Chunk) []byte {
	cache := c.networkEncoding()
	if cache.biomes == nil {
		cache.biomes = encodeBiomes(c, NetworkEncoding)
	}
	return cache.biomes
}

func (chunk *Chunk) networkEncoding() *networkEncodeCache {
	if !chunk.networkCache.valid {
		chunk.networkCache = networkEncodeCache{valid: true, subChunks: make([][]byte, len(chunk.sub))}
	}
	return &chunk.networkCache
}

func (chunk *Chunk) invalidateNetworkCache() {
	chunk.networkCache.valid = false
}

// encodePalettedStorage encodes a PalettedStorage into a bytes.Buffer. The Encoding passed is used to write the Palette
// of the PalettedStorage.
func encodePalettedStorage(buf *bytes.Buffer, storage, previous *PalettedStorage, e Encoding, pe paletteEncoding) {
	if storage.Equal(previous) {
		_, _ = buf.Write([]byte{0x7f<<1 | e.network()})
		return
	}
	b := make([]byte, len(storage.indices)*4+1)
	b[0] = byte(storage.bitsPerIndex<<1) | e.network()

	for i, v := range storage.indices {
		// Explicitly don't use the binary package to greatly improve performance of writing the uint32s.
		b[i*4+1], b[i*4+2], b[i*4+3], b[i*4+4] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
	}
	_, _ = buf.Write(b)

	e.encodePalette(buf, storage.palette, pe)
}
