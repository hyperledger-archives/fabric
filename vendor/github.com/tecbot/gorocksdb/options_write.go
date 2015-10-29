package gorocksdb

// #include "rocksdb/c.h"
import "C"

// WriteOptions represent all of the available options when writing to a
// database.
type WriteOptions struct {
	c *C.rocksdb_writeoptions_t
}

// NewDefaultWriteOptions creates a default WriteOptions object.
func NewDefaultWriteOptions() *WriteOptions {
	return NewNativeWriteOptions(C.rocksdb_writeoptions_create())
}

// NewNativeWriteOptions creates a WriteOptions object.
func NewNativeWriteOptions(c *C.rocksdb_writeoptions_t) *WriteOptions {
	return &WriteOptions{c}
}

// If true, the write will be flushed from the operating system
// buffer cache before the write is considered complete.
// If this flag is true, writes will be slower.
// Default: false
func (self *WriteOptions) SetSync(value bool) {
	C.rocksdb_writeoptions_set_sync(self.c, boolToChar(value))
}

// If true, writes will not first go to the write ahead log,
// and the write may got lost after a crash.
// Default: false
func (self *WriteOptions) DisableWAL(value bool) {
	C.rocksdb_writeoptions_disable_WAL(self.c, C.int(btoi(value)))
}

// Destroy deallocates the WriteOptions object.
func (self *WriteOptions) Destroy() {
	C.rocksdb_writeoptions_destroy(self.c)
	self.c = nil
}
