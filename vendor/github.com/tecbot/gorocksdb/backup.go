package gorocksdb

// #include <stdlib.h>
// #include "rocksdb/c.h"
import "C"

import (
	"errors"
	"unsafe"
)

// BackupEngineInfo represents the information about the backups
// in a backup engine instance. Use this get the state of the
// backup like number of backups and their ids and timestamps etc
type BackupEngineInfo struct {
	c *C.rocksdb_backup_engine_info_t
}

// GetCount gets the number backsup available
func (self *BackupEngineInfo) GetCount() int {
	return int(C.rocksdb_backup_engine_info_count(self.c))
}

// GetTimestamp gets the timestamp at which the backup @index was taken
func (self *BackupEngineInfo) GetTimestamp(index int) int64 {
	return int64(C.rocksdb_backup_engine_info_timestamp(self.c, C.int(index)))
}

// GetBackupId gets an id that uniquely identifies a backup
// regardless of its position
func (self *BackupEngineInfo) GetBackupId(index int) int64 {
	return int64(C.rocksdb_backup_engine_info_backup_id(self.c, C.int(index)))
}

// GetSize get the size of the backup in bytes
func (self *BackupEngineInfo) GetSize(index int) int64 {
	return int64(C.rocksdb_backup_engine_info_size(self.c, C.int(index)))
}

// GetNumFiles gets the number of files in the backup @index
func (self *BackupEngineInfo) GetNumFiles(index int) int32 {
	return int32(C.rocksdb_backup_engine_info_number_files(self.c, C.int(index)))
}

// Destroy destroys the backup engine info instance
func (self *BackupEngineInfo) Destroy() {
	C.rocksdb_backup_engine_info_destroy(self.c)
	self.c = nil
}

// RestoreOptions captures the options to be used during
// restoration of a backup
type RestoreOptions struct {
	c *C.rocksdb_restore_options_t
}

// NewRestoreOptions creates a RestoreOptions instance
func NewRestoreOptions() *RestoreOptions {
	return &RestoreOptions{
		c: C.rocksdb_restore_options_create(),
	}
}

// SetKeepLogFiles is used to set or unset the keep_log_files option
// If true, restore won't overwrite the existing log files in wal_dir. It will
// also move all log files from archive directory to wal_dir. By default, this
// is false
func (self *RestoreOptions) SetKeepLogFiles(v int) {
	C.rocksdb_restore_options_set_keep_log_files(self.c, C.int(v))
}

// Destroy destroys this RestoreOptions instance
func (self *RestoreOptions) Destroy() {
	C.rocksdb_restore_options_destroy(self.c)
}

// BackupEngine is a reusable handle to a RocksDB Backup, created by
// OpenBackupEngine
type BackupEngine struct {
	c    *C.rocksdb_backup_engine_t
	path string
	opts *Options
}

// OpenBackupEngine opens a backup engine with specified options
func OpenBackupEngine(opts *Options, path string) (*BackupEngine, error) {
	var cErr *C.char
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	be := C.rocksdb_backup_engine_open(opts.c, cpath, &cErr)
	if cErr != nil {
		defer C.free(unsafe.Pointer(cErr))
		return nil, errors.New(C.GoString(cErr))
	}

	return &BackupEngine{
		c:    be,
		path: path,
		opts: opts,
	}, nil
}

// CreateNewBackup takes a new backup from @db
func (self *BackupEngine) CreateNewBackup(db *DB) error {
	var cErr *C.char

	C.rocksdb_backup_engine_create_new_backup(self.c, db.c, &cErr)
	if cErr != nil {
		defer C.free(unsafe.Pointer(cErr))
		return errors.New(C.GoString(cErr))
	}

	return nil
}

// GetInfo gets an object that gives information about
// the backups that have already been taken
func (self *BackupEngine) GetInfo() *BackupEngineInfo {
	return &BackupEngineInfo{
		c: C.rocksdb_backup_engine_get_backup_info(self.c),
	}
}

// RestoreDBFromLatestBackup restores the latest backup to @db_dir. @wal_dir
// is where the write ahead logs are restored to and usually the same as @db_dir.
func (self *BackupEngine) RestoreDBFromLatestBackup(db_dir string, wal_dir string,
	opts *RestoreOptions) error {
	var cErr *C.char
	c_db_dir := C.CString(db_dir)
	c_wal_dir := C.CString(wal_dir)
	defer func() {
		C.free(unsafe.Pointer(c_db_dir))
		C.free(unsafe.Pointer(c_wal_dir))
	}()

	C.rocksdb_backup_engine_restore_db_from_latest_backup(self.c,
		c_db_dir, c_wal_dir, opts.c, &cErr)
	if cErr != nil {
		defer C.free(unsafe.Pointer(cErr))
		return errors.New(C.GoString(cErr))
	}

	return nil
}

// Close close the backup engine and cleans up state
// The backups already taken remain on storage.
func (self *BackupEngine) Close() {
	C.rocksdb_backup_engine_close(self.c)
	self.c = nil
}
