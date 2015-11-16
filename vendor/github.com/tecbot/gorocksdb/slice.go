package gorocksdb

// #include <stdlib.h>
import "C"

import "unsafe"

// Slice is used as a wrapper for non-copy values
type Slice struct {
	data  *C.char
	size  C.size_t
	freed bool
}

func NewSlice(data *C.char, size C.size_t) *Slice {
	return &Slice{data, size, false}
}

func (self *Slice) Data() []byte {
	return charToByte(self.data, self.size)
}

func (self *Slice) Size() int {
	return int(self.size)
}

func (self *Slice) Free() {
	if !self.freed {
		C.free(unsafe.Pointer(self.data))
		self.freed = true
	}
}
