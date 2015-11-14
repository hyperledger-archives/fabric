package gorocksdb

// #include "rocksdb/c.h"
import "C"

import (
	"io"
)

// WriteBatch is a batching of Puts, Merges and Deletes.
type WriteBatch struct {
	c *C.rocksdb_writebatch_t
}

// NewWriteBatch create a WriteBatch object.
func NewWriteBatch() *WriteBatch {
	return NewNativeWriteBatch(C.rocksdb_writebatch_create())
}

// NewNativeWriteBatch create a WriteBatch object.
func NewNativeWriteBatch(c *C.rocksdb_writebatch_t) *WriteBatch {
	return &WriteBatch{c}
}

// WriteBatchFrom creates a write batch from a serialized WriteBatch.
func WriteBatchFrom(data []byte) *WriteBatch {
	return NewNativeWriteBatch(C.rocksdb_writebatch_create_from(byteToChar(data), C.size_t(len(data))))
}

// Put queues a key-value pair.
func (self *WriteBatch) Put(key, value []byte) {
	cKey := byteToChar(key)
	cValue := byteToChar(value)

	C.rocksdb_writebatch_put(self.c, cKey, C.size_t(len(key)), cValue, C.size_t(len(value)))
}

// PutCF queues a key-value pair in a column family.
func (self *WriteBatch) PutCF(cf *ColumnFamilyHandle, key, value []byte) {
	cKey := byteToChar(key)
	cValue := byteToChar(value)

	C.rocksdb_writebatch_put_cf(self.c, cf.c, cKey, C.size_t(len(key)), cValue, C.size_t(len(value)))
}

// Merge queues a merge of "value" with the existing value of "key".
func (self *WriteBatch) Merge(key, value []byte) {
	cKey := byteToChar(key)
	cValue := byteToChar(value)

	C.rocksdb_writebatch_merge(self.c, cKey, C.size_t(len(key)), cValue, C.size_t(len(value)))
}

// MergeCF queues a merge of "value" with the existing value of "key" in a
// column family.
func (self *WriteBatch) MergeCF(cf *ColumnFamilyHandle, key, value []byte) {
	cKey := byteToChar(key)
	cValue := byteToChar(value)

	C.rocksdb_writebatch_merge_cf(self.c, cf.c, cKey, C.size_t(len(key)), cValue, C.size_t(len(value)))
}

// Delete queues a deletion of the data at key.
func (self *WriteBatch) Delete(key []byte) {
	cKey := byteToChar(key)

	C.rocksdb_writebatch_delete(self.c, cKey, C.size_t(len(key)))
}

// DeleteCF queues a deletion of the data at key in a column family.
func (self *WriteBatch) DeleteCF(cf *ColumnFamilyHandle, key []byte) {
	cKey := byteToChar(key)
	C.rocksdb_writebatch_delete_cf(self.c, cf.c, cKey, C.size_t(len(key)))
}

// Data returns the serialized version of this batch.
func (self *WriteBatch) Data() []byte {
	var cSize C.size_t
	cValue := C.rocksdb_writebatch_data(self.c, &cSize)

	return charToByte(cValue, cSize)
}

// Count returns the number of updates in the batch.
func (self *WriteBatch) Count() int {
	return int(C.rocksdb_writebatch_count(self.c))
}

func (self *WriteBatch) NewIterator() *WriteBatchIterator {
	data := self.Data()
	if len(data) < 8+4 {
		return &WriteBatchIterator{}
	}

	return &WriteBatchIterator{data: data[12:]}
}

// Clear removes all the enqueued Put and Deletes.
func (self *WriteBatch) Clear() {
	C.rocksdb_writebatch_clear(self.c)
}

// Destroy deallocates the WriteBatch object.
func (self *WriteBatch) Destroy() {
	C.rocksdb_writebatch_destroy(self.c)
	self.c = nil
}

type WriteBatchRecordType byte

const (
	WriteBatchRecordTypeDeletion WriteBatchRecordType = 0x0
	WriteBatchRecordTypeValue    WriteBatchRecordType = 0x1
	WriteBatchRecordTypeMerge    WriteBatchRecordType = 0x2
	WriteBatchRecordTypeLogData  WriteBatchRecordType = 0x3
)

type WriteBatchRecord struct {
	Key   []byte
	Value []byte
	Type  WriteBatchRecordType
}

type WriteBatchIterator struct {
	data   []byte
	record WriteBatchRecord
	err    error
}

func (self *WriteBatchIterator) Next() bool {
	if self.err != nil || len(self.data) == 0 {
		return false
	}

	self.record.Key = nil
	self.record.Value = nil

	recordType := WriteBatchRecordType(self.data[0])
	self.record.Type = recordType
	self.data = self.data[1:]

	x, n := self.decodeVarint(self.data)
	if n == 0 {
		self.err = io.ErrShortBuffer
		return false
	}
	k := n + int(x)
	self.record.Key = self.data[n:k]
	self.data = self.data[k:]

	if recordType == WriteBatchRecordTypeValue || recordType == WriteBatchRecordTypeMerge {
		x, n := self.decodeVarint(self.data)
		if n == 0 {
			self.err = io.ErrShortBuffer
			return false
		}
		k := n + int(x)
		self.record.Value = self.data[n:k]
		self.data = self.data[k:]
	}

	return true
}

func (self *WriteBatchIterator) Record() *WriteBatchRecord {
	return &self.record
}

func (self *WriteBatchIterator) Error() error {
	return self.err
}

func (self *WriteBatchIterator) decodeVarint(buf []byte) (x uint64, n int) {
	// x, n already 0
	for shift := uint(0); shift < 64; shift += 7 {
		if n >= len(buf) {
			return 0, 0
		}
		b := uint64(buf[n])
		n++
		x |= (b & 0x7F) << shift
		if (b & 0x80) == 0 {
			return x, n
		}
	}

	// The number is too large to represent in a 64-bit value.
	return 0, 0
}
