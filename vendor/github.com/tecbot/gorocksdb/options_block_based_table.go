package gorocksdb

// #include "rocksdb/c.h"
// #include "gorocksdb.h"
import "C"
import (
	"unsafe"
)

// Block-based table related options are moved to BlockBasedTableOptions.
// If you'd like to customize some of these options, you will need to
// use NewBlockBasedTableFactory() to construct a new table factory.
// For advanced user only
type BlockBasedTableOptions struct {
	c *C.rocksdb_block_based_table_options_t

	// hold references for GC
	cache      *Cache
	comp_cache *Cache
	fp         *FilterPolicy

	// Hold on so memory can be freed in Destroy.
	cfp *C.rocksdb_filterpolicy_t
}

// NewDefaultBlockBasedTableOptions creates a default BlockBasedTableOptions object.
func NewDefaultBlockBasedTableOptions() *BlockBasedTableOptions {
	return NewNativeBlockBasedTableOptions(C.rocksdb_block_based_options_create())
}

// NewNativeBlockBasedTableOptions creates a BlockBasedTableOptions object.
func NewNativeBlockBasedTableOptions(c *C.rocksdb_block_based_table_options_t) *BlockBasedTableOptions {
	return &BlockBasedTableOptions{c: c}
}

// Destroy deallocates the BlockBasedTableOptions object.
func (self *BlockBasedTableOptions) Destroy() {
	C.rocksdb_block_based_options_destroy(self.c)
	self.c = nil
	self.fp = nil
	self.cache = nil
	self.comp_cache = nil
}

// Approximate size of user data packed per block. Note that the
// block size specified here corresponds to uncompressed data. The
// actual size of the unit read from disk may be smaller if
// compression is enabled. This parameter can be changed dynamically.
// Default: 4K
func (self *BlockBasedTableOptions) SetBlockSize(block_size int) {
	C.rocksdb_block_based_options_set_block_size(self.c, C.size_t(block_size))
}

// This is used to close a block before it reaches the configured
// 'block_size'. If the percentage of free space in the current block is less
// than this specified number and adding a new record to the block will
// exceed the configured block size, then this block will be closed and the
// new record will be written to the next block.
// Default: 10
func (self *BlockBasedTableOptions) SetBlockSizeDeviation(block_size_deviation int) {
	C.rocksdb_block_based_options_set_block_size_deviation(self.c, C.int(block_size_deviation))
}

// Number of keys between restart points for delta encoding of keys.
// This parameter can be changed dynamically. Most clients should
// leave this parameter alone.
// Default: 16
func (self *BlockBasedTableOptions) SetBlockRestartInterval(block_restart_interval int) {
	C.rocksdb_block_based_options_set_block_restart_interval(self.c, C.int(block_restart_interval))
}

// If set use the specified filter policy to reduce disk reads.
// Many applications will benefit from passing the result of
// NewBloomFilterPolicy() here.
// Default: nil
func (self *BlockBasedTableOptions) SetFilterPolicy(value FilterPolicy) {
	if nfp, ok := value.(nativeFilterPolicy); ok {
		self.cfp = nfp.c
	} else {
		h := unsafe.Pointer(&value)
		self.fp = &value
		self.cfp = C.gorocksdb_filterpolicy_create(h)
	}
	C.rocksdb_block_based_options_set_filter_policy(self.c, self.cfp)
}

// Disable block cache. If this is set to true, then no block cache
// should be used.
// Default: false
func (self *BlockBasedTableOptions) SetNoBlockCache(value bool) {
	C.rocksdb_block_based_options_set_no_block_cache(self.c, boolToChar(value))
}

// Control over blocks (user data is stored in a set of blocks, and
// a block is the unit of reading from disk).
//
// If set, use the specified cache for blocks.
// If nil, rocksdb will automatically create and use an 8MB internal cache.
// Default: nil
func (self *BlockBasedTableOptions) SetBlockCache(value *Cache) {
	self.cache = value

	C.rocksdb_block_based_options_set_block_cache(self.c, value.c)
}

// If set, use the specified cache for compressed blocks.
// If nil, rocksdb will not use a compressed block cache.
// Default: nil
func (self *BlockBasedTableOptions) SetBlockCacheCompressed(value *Cache) {
	self.comp_cache = value

	C.rocksdb_block_based_options_set_block_cache_compressed(self.c, value.c)
}

// If true, place whole keys in the filter (not just prefixes).
// This must generally be true for gets to be efficient.
// Default: true
func (self *BlockBasedTableOptions) SetWholeKeyFiltering(value bool) {
	C.rocksdb_block_based_options_set_whole_key_filtering(self.c, boolToChar(value))
}
