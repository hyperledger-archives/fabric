package gorocksdb

// #include "rocksdb/c.h"
import "C"

// An application can issue a read request (via Get/Iterators) and specify
// if that read should process data that ALREADY resides on a specified cache
// level. For example, if an application specifies BlockCacheTier then the
// Get call will process data that is already processed in the memtable or
// the block cache. It will not page in data from the OS cache or data that
// resides in storage.
type ReadTier uint

const (
	// data in memtable, block cache, OS cache or storage
	ReadAllTier = ReadTier(0)
	// data in memtable or block cache
	BlockCacheTier = ReadTier(1)
)

// ReadOptions represent all of the available options when reading from a
// database.
type ReadOptions struct {
	c *C.rocksdb_readoptions_t
}

// NewDefaultReadOptions creates a default ReadOptions object.
func NewDefaultReadOptions() *ReadOptions {
	return NewNativeReadOptions(C.rocksdb_readoptions_create())
}

// NewNativeReadOptions creates a ReadOptions object.
func NewNativeReadOptions(c *C.rocksdb_readoptions_t) *ReadOptions {
	return &ReadOptions{c}
}

// If true, all data read from underlying storage will be
// verified against corresponding checksums.
// Default: false
func (self *ReadOptions) SetVerifyChecksums(value bool) {
	C.rocksdb_readoptions_set_verify_checksums(self.c, boolToChar(value))
}

// Should the "data block"/"index block"/"filter block" read for this
// iteration be cached in memory?
// Callers may wish to set this field to false for bulk scans.
// Default: true
func (self *ReadOptions) SetFillCache(value bool) {
	C.rocksdb_readoptions_set_fill_cache(self.c, boolToChar(value))
}

// If snapshot is set, read as of the supplied snapshot
// which must belong to the DB that is being read and which must
// not have been released.
// Default: nil
func (self *ReadOptions) SetSnapshot(snap *Snapshot) {
	C.rocksdb_readoptions_set_snapshot(self.c, snap.c)
}

// Specify if this read request should process data that ALREADY
// resides on a particular cache. If the required data is not
// found at the specified cache, then Status::Incomplete is returned.
// Default: ReadAllTier
func (self *ReadOptions) SetReadTier(value ReadTier) {
	C.rocksdb_readoptions_set_read_tier(self.c, C.int(value))
}

// Specify to create a tailing iterator -- a special iterator that has a
// view of the complete database (i.e. it can also be used to read newly
// added data) and is optimized for sequential reads. It will return records
// that were inserted into the database after the creation of the iterator.
// Default: false
func (self *ReadOptions) SetTailing(value bool) {
	C.rocksdb_readoptions_set_tailing(self.c, boolToChar(value))
}

// Destroy deallocates the ReadOptions object.
func (self *ReadOptions) Destroy() {
	C.rocksdb_readoptions_destroy(self.c)
	self.c = nil
}
