package gorocksdb

// #include "rocksdb/c.h"
import "C"

// Env is a system call environment used by a database.
type Env struct {
	c *C.rocksdb_env_t
}

// NewDefaultEnv creates a default environment.
func NewDefaultEnv() *Env {
	return NewNativeEnv(C.rocksdb_create_default_env())
}

// NewNativeEnv creates a Environment object.
func NewNativeEnv(c *C.rocksdb_env_t) *Env {
	return &Env{c}
}

// The number of background worker threads of a specific thread pool
// for this environment. 'LOW' is the default pool.
// Default: 1
func (self *Env) SetBackgroundThreads(n int) {
	C.rocksdb_env_set_background_threads(self.c, C.int(n))
}

// SetHighPriorityBackgroundThreads sets the size of the high priority
// thread pool that can be used to prevent compactions from stalling
// memtable flushes.
func (self *Env) SetHighPriorityBackgroundThreads(n int) {
	C.rocksdb_env_set_high_priority_background_threads(self.c, C.int(n))
}

// Destroy deallocates the Env object.
func (self *Env) Destroy() {
	C.rocksdb_env_destroy(self.c)
	self.c = nil
}
