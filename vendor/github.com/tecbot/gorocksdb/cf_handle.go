package gorocksdb

// #include <stdlib.h>
// #include "rocksdb/c.h"
import "C"

type ColumnFamilyHandle struct {
	c *C.rocksdb_column_family_handle_t
}

// NewNativeColumnFamilyHandle creates a ColumnFamilyHandle object.
func NewNativeColumnFamilyHandle(c *C.rocksdb_column_family_handle_t) *ColumnFamilyHandle {
	return &ColumnFamilyHandle{c}
}

func (h *ColumnFamilyHandle) Destroy() {
	C.rocksdb_column_family_handle_destroy(h.c)
}
