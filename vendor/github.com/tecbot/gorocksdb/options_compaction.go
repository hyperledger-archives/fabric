package gorocksdb

// #include "rocksdb/c.h"
import "C"

// Algorithm used to make a compaction request stop picking new files
// into a single compaction run.
type UniversalCompactionStopStyle uint

const (
	CompactionStopStyleSimilarSize = UniversalCompactionStopStyle(0)
	CompactionStopStyleTotalSize   = UniversalCompactionStopStyle(1)
)

// FIFOCompactionOptions represent all of the available options for
// FIFO compaction.
type FIFOCompactionOptions struct {
	c *C.rocksdb_fifo_compaction_options_t
}

// NewDefaultFIFOCompactionOptions creates a default FIFOCompactionOptions object.
func NewDefaultFIFOCompactionOptions() *FIFOCompactionOptions {
	return NewNativeFIFOCompactionOptions(C.rocksdb_fifo_compaction_options_create())
}

// NewDefaultFIFOCompactionOptions creates a default FIFOCompactionOptions object.
func NewNativeFIFOCompactionOptions(c *C.rocksdb_fifo_compaction_options_t) *FIFOCompactionOptions {
	return &FIFOCompactionOptions{c}
}

// Once the total sum of table files reaches this, we will delete the oldest
// table file
// Default: 1GB
func (self *FIFOCompactionOptions) SetMaxTableFilesSize(value uint64) {
	C.rocksdb_fifo_compaction_options_set_max_table_files_size(self.c, C.uint64_t(value))
}

// Destroy deallocates the FIFOCompactionOptions object.
func (self *FIFOCompactionOptions) Destroy() {
	C.rocksdb_fifo_compaction_options_destroy(self.c)
}

// UniversalCompactionOptions represent all of the available options for
// universal compaction.
type UniversalCompactionOptions struct {
	c *C.rocksdb_universal_compaction_options_t
}

// NewDefaultUniversalCompactionOptions creates a default UniversalCompactionOptions
// object.
func NewDefaultUniversalCompactionOptions() *UniversalCompactionOptions {
	return NewNativeUniversalCompactionOptions(C.rocksdb_universal_compaction_options_create())
}

// NewNativeUniversalCompactionOptions creates a UniversalCompactionOptions
// object.
func NewNativeUniversalCompactionOptions(c *C.rocksdb_universal_compaction_options_t) *UniversalCompactionOptions {
	return &UniversalCompactionOptions{c}
}

// Percentage flexibilty while comparing file size. If the candidate file(s)
// size is 1% smaller than the next file's size, then include next file into
// this candidate set.
// Default: 1
func (self *UniversalCompactionOptions) SetSizeRatio(value uint) {
	C.rocksdb_universal_compaction_options_set_size_ratio(self.c, C.int(value))
}

// The minimum number of files in a single compaction run.
// Default: 2
func (self *UniversalCompactionOptions) SetMinMergeWidth(value uint) {
	C.rocksdb_universal_compaction_options_set_min_merge_width(self.c, C.int(value))
}

// The maximum number of files in a single compaction run.
// Default: UINT_MAX
func (self *UniversalCompactionOptions) SetMaxMergeWidth(value uint) {
	C.rocksdb_universal_compaction_options_set_max_merge_width(self.c, C.int(value))
}

// The size amplification is defined as the amount (in percentage) of
// additional storage needed to store a single byte of data in the database.
// For example, a size amplification of 2% means that a database that
// contains 100 bytes of user-data may occupy upto 102 bytes of
// physical storage. By this definition, a fully compacted database has
// a size amplification of 0%. Rocksdb uses the following heuristic
// to calculate size amplification: it assumes that all files excluding
// the earliest file contribute to the size amplification.
// Default: 200, which means that a 100 byte database could require upto
// 300 bytes of storage.
func (self *UniversalCompactionOptions) SetMaxSizeAmplificationPercent(value uint) {
	C.rocksdb_universal_compaction_options_set_max_size_amplification_percent(self.c, C.int(value))
}

// If this option is set to be -1, all the output files
// will follow compression type specified.
//
// If this option is not negative, we will try to make sure compressed
// size is just above this value. In normal cases, at least this percentage
// of data will be compressed.
// When we are compacting to a new file, here is the criteria whether
// it needs to be compressed: assuming here are the list of files sorted
// by generation time:
//    A1...An B1...Bm C1...Ct
// where A1 is the newest and Ct is the oldest, and we are going to compact
// B1...Bm, we calculate the total size of all the files as total_size, as
// well as  the total size of C1...Ct as total_C, the compaction output file
// will be compressed iff
//   total_C / total_size < this percentage
// Default: -1
func (self *UniversalCompactionOptions) SetCompressionSizePercent(value int) {
	C.rocksdb_universal_compaction_options_set_compression_size_percent(self.c, C.int(value))
}

// The algorithm used to stop picking files into a single compaction run
// Default: CompactionStopStyleTotalSize
func (self *UniversalCompactionOptions) SetStopStyle(value UniversalCompactionStopStyle) {
	C.rocksdb_universal_compaction_options_set_stop_style(self.c, C.int(value))
}

// Destroy deallocates the UniversalCompactionOptions object.
func (self *UniversalCompactionOptions) Destroy() {
	C.rocksdb_universal_compaction_options_destroy(self.c)
	self.c = nil
}
