package gorocksdb

// #include "rocksdb/c.h"
import "C"

// A SliceTransform can be used as a prefix extractor.
type SliceTransform interface {
	// Transform a src in domain to a dst in the range.
	Transform(src []byte) []byte

	// Determine whether this is a valid src upon the function applies.
	InDomain(src []byte) bool

	// Determine whether dst=Transform(src) for some src.
	InRange(src []byte) bool

	// Return the name of this transformation.
	Name() string
}

// This type is a bit of a hack and will not behave as expected if clients try to
// call its methods. It is handled specially in Options.
type nativeSliceTransform struct {
	c *C.rocksdb_slicetransform_t
}

func (st nativeSliceTransform) Transform(src []byte) []byte { return nil }

func (st nativeSliceTransform) InDomain(src []byte) bool { return false }

func (st nativeSliceTransform) InRange(src []byte) bool { return false }

func (st nativeSliceTransform) Name() string { return "" }

// NewFixedPrefixTransform creates a new fixed prefix transform.
func NewFixedPrefixTransform(prefixLen int) SliceTransform {
	return NewNativeSliceTransform(C.rocksdb_slicetransform_create_fixed_prefix(C.size_t(prefixLen)))
}

// NewNativeSliceTransform allocates a SliceTransform object.
// The SliceTransform's methods are no-ops, but it is still used correctly by
// RocksDB.
func NewNativeSliceTransform(c *C.rocksdb_slicetransform_t) SliceTransform {
	return nativeSliceTransform{c}
}

//export gorocksdb_slicetransform_transform
func gorocksdb_slicetransform_transform(handler *SliceTransform, cKey *C.char, cKeyLen C.size_t, cDstLen *C.size_t) *C.char {
	key := charToByte(cKey, cKeyLen)

	dst := (*handler).Transform(key)

	*cDstLen = C.size_t(len(dst))

	return byteToChar(dst)
}

//export gorocksdb_slicetransform_in_domain
func gorocksdb_slicetransform_in_domain(handler *SliceTransform, cKey *C.char, cKeyLen C.size_t) C.uchar {
	key := charToByte(cKey, cKeyLen)

	inDomain := (*handler).InDomain(key)

	return boolToChar(inDomain)
}

//export gorocksdb_slicetransform_in_range
func gorocksdb_slicetransform_in_range(handler *SliceTransform, cKey *C.char, cKeyLen C.size_t) C.uchar {
	key := charToByte(cKey, cKeyLen)

	inRange := (*handler).InRange(key)

	return boolToChar(inRange)
}

//export gorocksdb_slicetransform_name
func gorocksdb_slicetransform_name(handler *SliceTransform) *C.char {
	return stringToChar((*handler).Name())
}
