package gorocksdb

// #include "rocksdb/c.h"
// #include "gorocksdb.h"
import "C"
import (
	"unsafe"
)

// DB contents are stored in a set of blocks, each of which holds a
// sequence of key,value pairs. Each block may be compressed before
// being stored in a file. The following enum describes which
// compression method (if any) is used to compress a block.
type CompressionType uint

const (
	NoCompression     = CompressionType(0)
	SnappyCompression = CompressionType(1)
	ZlibCompression   = CompressionType(2)
	BZip2Compression  = CompressionType(3)
)

type CompactionStyle uint

const (
	LevelCompactionStyle     = CompactionStyle(0)
	UniversalCompactionStyle = CompactionStyle(1)
	FIFOCompactionStyle      = CompactionStyle(2)
)

type CompactionAccessPattern uint

const (
	NoneCompactionAccessPattern       = CompactionAccessPattern(0)
	NormalCompactionAccessPattern     = CompactionAccessPattern(1)
	SequentialCompactionAccessPattern = CompactionAccessPattern(2)
	WillneedCompactionAccessPattern   = CompactionAccessPattern(3)
)

type InfoLogLevel uint

const (
	DebugInfoLogLevel = InfoLogLevel(0)
	InfoInfoLogLevel  = InfoLogLevel(1)
	WarnInfoLogLevel  = InfoLogLevel(2)
	ErrorInfoLogLevel = InfoLogLevel(3)
	FatalInfoLogLevel = InfoLogLevel(4)
)

// Options represent all of the available options when opening a database with Open.
type Options struct {
	c *C.rocksdb_options_t

	// hold references for GC
	cmp  *Comparator
	mo   *MergeOperator
	env  *Env
	st   *SliceTransform
	cf   *CompactionFilter
	bbto *BlockBasedTableOptions

	// We keep these so we can free their memory in Destroy.
	ccmp *C.rocksdb_comparator_t
	cmo  *C.rocksdb_mergeoperator_t
	cst  *C.rocksdb_slicetransform_t
	ccf  *C.rocksdb_compactionfilter_t
}

// NewDefaultOptions creates the default Options.
func NewDefaultOptions() *Options {
	return NewNativeOptions(C.rocksdb_options_create())
}

// NewNativeOptions creates a Options object.
func NewNativeOptions(c *C.rocksdb_options_t) *Options {
	return &Options{c: c}
}

// -------------------
// Parameters that affect behavior

// If set, the specified compaction filter will be applied
// on compactions.
// Default: nil
func (self *Options) SetCompactionFilter(filter CompactionFilter) {
	h := unsafe.Pointer(&filter)
	self.cf = &filter
	self.ccf = C.gorocksdb_compactionfilter_create(h)
	C.rocksdb_options_set_compaction_filter(self.c, self.ccf)
}

// Comparator used to define the order of keys in the table.
// Default: a comparator that uses lexicographic byte-wise ordering
func (self *Options) SetComparator(value Comparator) {
	if nc, ok := value.(nativeComparator); ok {
		self.ccmp = nc.c
	} else {
		h := unsafe.Pointer(&value)
		self.cmp = &value
		self.ccmp = C.gorocksdb_comparator_create(h)
	}
	C.rocksdb_options_set_comparator(self.c, self.ccmp)
}

// The merge operator will called if Merge operations are used.
// Default: nil
func (self *Options) SetMergeOperator(value MergeOperator) {
	if nmo, ok := value.(nativeMergeOperator); ok {
		self.cmo = nmo.c
	} else {
		h := unsafe.Pointer(&value)
		self.mo = &value
		self.cmo = C.gorocksdb_mergeoperator_create(h)
	}
	C.rocksdb_options_set_merge_operator(self.c, self.cmo)
}

// A single CompactionFilter instance to call into during compaction.
// Allows an application to modify/delete a key-value during background
// compaction.
//
// If the client requires a new compaction filter to be used for different
// compaction runs, it can specify compaction_filter_factory instead of this
// option. The client should specify only one of the two.
// compaction_filter takes precedence over compaction_filter_factory if
// client specifies both.
//
// If multithreaded compaction is being used, the supplied CompactionFilter
// instance may be used from different threads concurrently and so should be
// thread-safe.
//
// Default: nil
// TODO: implement in C
//func (self *Options) SetCompactionFilter(value *CompactionFilter) {
//	C.rocksdb_options_set_compaction_filter(self.c, value.filter)
//}

// This is a factory that provides compaction filter objects which allow
// an application to modify/delete a key-value during background compaction.
//
// A new filter will be created on each compaction run.  If multithreaded
// compaction is being used, each created CompactionFilter will only be used
// from a single thread and so does not need to be thread-safe.
//
// Default: a factory that doesn't provide any object
// std::shared_ptr<CompactionFilterFactory> compaction_filter_factory;
// TODO: implement in C and Go

// Version TWO of the compaction_filter_factory
// It supports rolling compaction
//
// Default: a factory that doesn't provide any object
// std::shared_ptr<CompactionFilterFactoryV2> compaction_filter_factory_v2;
// TODO: implement in C and Go

// If true, the database will be created if it is missing.
// Default: false
func (self *Options) SetCreateIfMissing(value bool) {
	C.rocksdb_options_set_create_if_missing(self.c, boolToChar(value))
}

// If true, an error is raised if the database already exists.
// Default: false
func (self *Options) SetErrorIfExists(value bool) {
	C.rocksdb_options_set_error_if_exists(self.c, boolToChar(value))
}

// If true, the implementation will do aggressive checking of the
// data it is processing and will stop early if it detects any
// errors. This may have unforeseen ramifications: for example, a
// corruption of one DB entry may cause a large number of entries to
// become unreadable or for the entire DB to become unopenable.
// If any of the  writes to the database fails (Put, Delete, Merge, Write),
// the database will switch to read-only mode and fail all other
// Write operations.
// Default: false
func (self *Options) SetParanoidChecks(value bool) {
	C.rocksdb_options_set_paranoid_checks(self.c, boolToChar(value))
}

// Use the specified object to interact with the environment,
// e.g. to read/write files, schedule background work, etc.
// Default: DefaultEnv
func (self *Options) SetEnv(value *Env) {
	self.env = value

	C.rocksdb_options_set_env(self.c, value.c)
}

// Sets the info log level.
// Default: InfoInfoLogLevel
func (self *Options) SetInfoLogLevel(value InfoLogLevel) {
	C.rocksdb_options_set_info_log_level(self.c, C.int(value))
}

// -------------------
// Parameters that affect performance

// By default, RocksDB uses only one background thread for flush and
// compaction. Calling this function will set it up such that total of
// `total_threads` is used. Good value for `total_threads` is the number of
// cores. You almost definitely want to call this function if your system is
// bottlenecked by RocksDB.
func (self *Options) IncreaseParallelism(total_threads int) {
	C.rocksdb_options_increase_parallelism(self.c, C.int(total_threads))
}

// Use this if you don't need to keep the data sorted, i.e. you'll never use
// an iterator, only Put() and Get() API calls
func (self *Options) OptimizeForPointLookup(block_cache_size_mb uint64) {
	C.rocksdb_options_optimize_for_point_lookup(self.c, C.uint64_t(block_cache_size_mb))
}

// Default values for some parameters in ColumnFamilyOptions are not
// optimized for heavy workloads and big datasets, which means you might
// observe write stalls under some conditions. As a starting point for tuning
// RocksDB options, use the following two functions:
// * OptimizeLevelStyleCompaction -- optimizes level style compaction
// * OptimizeUniversalStyleCompaction -- optimizes universal style compaction
// Universal style compaction is focused on reducing Write Amplification
// Factor for big data sets, but increases Space Amplification. You can learn
// more about the different styles here:
// https://github.com/facebook/rocksdb/wiki/Rocksdb-Architecture-Guide
// Make sure to also call IncreaseParallelism(), which will provide the
// biggest performance gains.
// Note: we might use more memory than memtable_memory_budget during high
// write rate period
func (self *Options) OptimizeLevelStyleCompaction(memtable_memory_budget uint64) {
	C.rocksdb_options_optimize_level_style_compaction(self.c, C.uint64_t(memtable_memory_budget))
}

// See note on OptimizeLevelStyleCompaction.
func (self *Options) OptimizeUniversalStyleCompaction(memtable_memory_budget uint64) {
	C.rocksdb_options_optimize_universal_style_compaction(self.c, C.uint64_t(memtable_memory_budget))
}

// Amount of data to build up in memory (backed by an unsorted log
// on disk) before converting to a sorted on-disk file.
//
// Larger values increase performance, especially during bulk loads.
// Up to max_write_buffer_number write buffers may be held in memory
// at the same time,
// so you may wish to adjust this parameter to control memory usage.
// Also, a larger write buffer will result in a longer recovery time
// the next time the database is opened.
// Default: 4MB
func (self *Options) SetWriteBufferSize(value int) {
	C.rocksdb_options_set_write_buffer_size(self.c, C.size_t(value))
}

// The maximum number of write buffers that are built up in memory.
// The default is 2, so that when 1 write buffer is being flushed to
// storage, new writes can continue to the other write buffer.
// Default: 2
func (self *Options) SetMaxWriteBufferNumber(value int) {
	C.rocksdb_options_set_max_write_buffer_number(self.c, C.int(value))
}

// The minimum number of write buffers that will be merged together
// before writing to storage. If set to 1, then
// all write buffers are flushed to L0 as individual files and this increases
// read amplification because a get request has to check in all of these
// files. Also, an in-memory merge may result in writing lesser
// data to storage if there are duplicate records in each of these
// individual write buffers.
// Default: 1
func (self *Options) SetMinWriteBufferNumberToMerge(value int) {
	C.rocksdb_options_set_min_write_buffer_number_to_merge(self.c, C.int(value))
}

// Number of open files that can be used by the DB. You may need to
// increase this if your database has a large working set (budget
// one open file per 2MB of working set).
// Default: 1000
func (self *Options) SetMaxOpenFiles(value int) {
	C.rocksdb_options_set_max_open_files(self.c, C.int(value))
}

// Compress blocks using the specified compression algorithm. This
// parameter can be changed dynamically.
//
// Default: SnappyCompression, which gives lightweight but fast
// compression.
func (self *Options) SetCompression(value CompressionType) {
	C.rocksdb_options_set_compression(self.c, C.int(value))
}

// Different levels can have different compression policies. There
// are cases where most lower levels would like to quick compression
// algorithm while the higher levels (which have more data) use
// compression algorithms that have better compression but could
// be slower. This array should have an entry for
// each level of the database. This array overrides the
// value specified in the previous field 'compression'.
func (self *Options) SetCompressionPerLevel(value []CompressionType) {
	cLevels := make([]C.int, len(value))
	for i, v := range value {
		cLevels[i] = C.int(v)
	}

	C.rocksdb_options_set_compression_per_level(self.c, &cLevels[0], C.size_t(len(value)))
}

// Sets the start level to use compression.
func (self *Options) SetMinLevelToCompress(value int) {
	C.rocksdb_options_set_min_level_to_compress(self.c, C.int(value))
}

// Sets different options for compression algorithms.
// Default: nil
func (self *Options) SetCompressionOptions(value *CompressionOptions) {
	C.rocksdb_options_set_compression_options(self.c, C.int(value.WindowBits), C.int(value.Level), C.int(value.Strategy))
}

// If set, use the specified function to determine the
// prefixes for keys. These prefixes will be placed in the filter.
// Depending on the workload, this can reduce the number of read-IOP
// cost for scans when a prefix is passed via ReadOptions to
// db.NewIterator().
// Default: nil
func (self *Options) SetPrefixExtractor(value SliceTransform) {
	if nst, ok := value.(nativeSliceTransform); ok {
		self.cst = nst.c
	} else {
		h := unsafe.Pointer(&value)
		self.st = &value
		self.cst = C.gorocksdb_slicetransform_create(h)
	}
	C.rocksdb_options_set_prefix_extractor(self.c, self.cst)
}

// Number of levels for this database.
// Default: 7
func (self *Options) SetNumLevels(value int) {
	C.rocksdb_options_set_num_levels(self.c, C.int(value))
}

// Number of files to trigger level-0 compaction. A value <0 means that
// level-0 compaction will not be triggered by number of files at all.
// Default: 4
func (self *Options) SetLevel0FileNumCompactionTrigger(value int) {
	C.rocksdb_options_set_level0_file_num_compaction_trigger(self.c, C.int(value))
}

// Soft limit on number of level-0 files. We start slowing down writes at this
// point. A value <0 means that no writing slow down will be triggered by
// number of files in level-0.
// Default: 8
func (self *Options) SetLevel0SlowdownWritesTrigger(value int) {
	C.rocksdb_options_set_level0_slowdown_writes_trigger(self.c, C.int(value))
}

// Maximum number of level-0 files.  We stop writes at this point.
// Default: 12
func (self *Options) SetLevel0StopWritesTrigger(value int) {
	C.rocksdb_options_set_level0_stop_writes_trigger(self.c, C.int(value))
}

// Maximum level to which a new compacted memtable is pushed if it
// does not create overlap. We try to push to level 2 to avoid the
// relatively expensive level 0=>1 compactions and to avoid some
// expensive manifest file operations. We do not push all the way to
// the largest level since that can generate a lot of wasted disk
// space if the same key space is being repeatedly overwritten.
// Default: 2
func (self *Options) SetMaxMemCompactionLevel(value int) {
	C.rocksdb_options_set_max_mem_compaction_level(self.c, C.int(value))
}

// Target file size for compaction, is per-file size for level-1.
// Target file size for level L can be calculated by
// target_file_size_base * (target_file_size_multiplier ^ (L-1))
//
// For example, if target_file_size_base is 2MB and
// target_file_size_multiplier is 10, then each file on level-1 will
// be 2MB, and each file on level 2 will be 20MB,
// and each file on level-3 will be 200MB.
// Default: 2MB
func (self *Options) SetTargetFileSizeBase(value uint64) {
	C.rocksdb_options_set_target_file_size_base(self.c, C.uint64_t(value))
}

// Target file size multiplier for compaction.
// Default: 1
func (self *Options) SetTargetFileSizeMultiplier(value int) {
	C.rocksdb_options_set_target_file_size_multiplier(self.c, C.int(value))
}

// Control maximum total data size for a level, is the max total for level-1.
// Maximum number of bytes for level L can be calculated as
// (max_bytes_for_level_base) * (max_bytes_for_level_multiplier ^ (L-1))
//
// For example, if max_bytes_for_level_base is 20MB, and if
// max_bytes_for_level_multiplier is 10, total data size for level-1
// will be 20MB, total file size for level-2 will be 200MB,
// and total file size for level-3 will be 2GB.
// Default: 10MB
func (self *Options) SetMaxBytesForLevelBase(value uint64) {
	C.rocksdb_options_set_max_bytes_for_level_base(self.c, C.uint64_t(value))
}

// Max Bytes for level multiplier.
// Default: 10
func (self *Options) SetMaxBytesForLevelMultiplier(value int) {
	C.rocksdb_options_set_max_bytes_for_level_multiplier(self.c, C.int(value))
}

// Different max-size multipliers for different levels.
// These are multiplied by max_bytes_for_level_multiplier to arrive
// at the max-size of each level.
// Default: 1 for each level
func (self *Options) SetMaxBytesForLevelMultiplierAdditional(value []int) {
	cLevels := make([]C.int, len(value))
	for i, v := range value {
		cLevels[i] = C.int(v)
	}

	C.rocksdb_options_set_max_bytes_for_level_multiplier_additional(self.c, &cLevels[0], C.size_t(len(value)))
}

// Maximum number of bytes in all compacted files. We avoid expanding
// the lower level file set of a compaction if it would make the
// total compaction cover more than
// (expanded_compaction_factor * targetFileSizeLevel()) many bytes.
// Default: 25
func (self *Options) SetExpandedCompactionFactor(value int) {
	C.rocksdb_options_set_expanded_compaction_factor(self.c, C.int(value))
}

// Maximum number of bytes in all source files to be compacted in a
// single compaction run. We avoid picking too many files in the
// source level so that we do not exceed the total source bytes
// for compaction to exceed
// (source_compaction_factor * targetFileSizeLevel()) many bytes.
// Default: 1
func (self *Options) SetSourceCompactionFactor(value int) {
	C.rocksdb_options_set_source_compaction_factor(self.c, C.int(value))
}

// Control maximum bytes of overlaps in grandparent (i.e., level+2) before we
// stop building a single file in a level->level+1 compaction.
// Default: 10
func (self *Options) SetMaxGrandparentOverlapFactor(value int) {
	C.rocksdb_options_set_max_grandparent_overlap_factor(self.c, C.int(value))
}

// If true, then the contents of data files are not synced
// to stable storage. Their contents remain in the OS buffers till the
// OS decides to flush them. This option is good for bulk-loading
// of data. Once the bulk-loading is complete, please issue a
// sync to the OS to flush all dirty buffers to stable storage.
// Default: false
func (self *Options) SetDisableDataSync(value bool) {
	C.rocksdb_options_set_disable_data_sync(self.c, C.int(btoi(value)))
}

// If true, then every store to stable storage will issue a fsync.
// If false, then every store to stable storage will issue a fdatasync.
// This parameter should be set to true while storing data to
// filesystem like ext3 that can lose files after a reboot.
// Default: false
func (self *Options) SetUseFsync(value bool) {
	C.rocksdb_options_set_use_fsync(self.c, C.int(btoi(value)))
}

// This specifies the absolute info LOG dir.
// If it is empty, the log files will be in the same dir as data.
// If it is non empty, the log files will be in the specified dir,
// and the db data dir's absolute path will be used as the log file
// name's prefix.
// Default: empty
func (self *Options) SetDbLogDir(value string) {
	cvalue := C.CString(value)
	defer C.free(unsafe.Pointer(cvalue))
	C.rocksdb_options_set_db_log_dir(self.c, cvalue)
}

// This specifies the absolute dir path for write-ahead logs (WAL).
// If it is empty, the log files will be in the same dir as data.
// If it is non empty, the log files will be in the specified dir,
// When destroying the db, all log files and the dir itself is deleted.
// Default: empty
func (self *Options) SetWalDir(value string) {
	cvalue := C.CString(value)
	defer C.free(unsafe.Pointer(cvalue))
	C.rocksdb_options_set_wal_dir(self.c, cvalue)
}

// The periodicity when obsolete files get deleted.
// The files that get out of scope by compaction
// process will still get automatically delete on every compaction,
// regardless of this setting.
// Default: 6 hours
func (self *Options) SetDeleteObsoleteFilesPeriodMicros(value uint64) {
	C.rocksdb_options_set_delete_obsolete_files_period_micros(self.c, C.uint64_t(value))
}

// Maximum number of concurrent background jobs, submitted to
// the default LOW priority thread pool
// Default: 1
func (self *Options) SetMaxBackgroundCompactions(value int) {
	C.rocksdb_options_set_max_background_compactions(self.c, C.int(value))
}

// Maximum number of concurrent background memtable flush jobs, submitted to
// the HIGH priority thread pool.
// By default, all background jobs (major compaction and memtable flush) go
// to the LOW priority pool. If this option is set to a positive number,
// memtable flush jobs will be submitted to the HIGH priority pool.
// It is important when the same Env is shared by multiple db instances.
// Without a separate pool, long running major compaction jobs could
// potentially block memtable flush jobs of other db instances, leading to
// unnecessary Put stalls.
// Default: 0
func (self *Options) SetMaxBackgroundFlushes(value int) {
	C.rocksdb_options_set_max_background_flushes(self.c, C.int(value))
}

// Specify the maximal size of the info log file. If the log file
// is larger than `max_log_file_size`, a new info log file will be created.
// If max_log_file_size == 0, all logs will be written to one log file.
// Default: 0
func (self *Options) SetMaxLogFileSize(value int) {
	C.rocksdb_options_set_max_log_file_size(self.c, C.size_t(value))
}

// Time for the info log file to roll (in seconds).
// If specified with non-zero value, log file will be rolled
// if it has been active longer than `log_file_time_to_roll`.
// Default: 0 (disabled)
func (self *Options) SetLogFileTimeToRoll(value int) {
	C.rocksdb_options_set_log_file_time_to_roll(self.c, C.size_t(value))
}

// Maximal info log files to be kept.
// Default: 1000
func (self *Options) SetKeepLogFileNum(value int) {
	C.rocksdb_options_set_keep_log_file_num(self.c, C.size_t(value))
}

// Puts are delayed 0-1 ms when any level has a compaction score that exceeds
// soft_rate_limit. This is ignored when == 0.0.
// CONSTRAINT: soft_rate_limit <= hard_rate_limit. If this constraint does not
// hold, RocksDB will set soft_rate_limit = hard_rate_limit
// Default: 0.0 (disabled)
func (self *Options) SetSoftRateLimit(value float64) {
	C.rocksdb_options_set_soft_rate_limit(self.c, C.double(value))
}

// Puts are delayed 1ms at a time when any level has a compaction score that
// exceeds hard_rate_limit. This is ignored when <= 1.0.
// Default: 0.0 (disabled)
func (self *Options) SetHardRateLimit(value float64) {
	C.rocksdb_options_set_hard_rate_limit(self.c, C.double(value))
}

// Max time a put will be stalled when hard_rate_limit is enforced. If 0, then
// there is no limit.
// Default: 1000
func (self *Options) SetRateLimitDelayMaxMilliseconds(value uint) {
	C.rocksdb_options_set_rate_limit_delay_max_milliseconds(self.c, C.uint(value))
}

// manifest file is rolled over on reaching this limit.
// The older manifest file be deleted.
// Default: MAX_INT so that roll-over does not take place.
func (self *Options) SetMaxManifestFileSize(value uint64) {
	C.rocksdb_options_set_max_manifest_file_size(self.c, C.size_t(value))
}

// Number of shards used for table cache.
// Default: 4
func (self *Options) SetTableCacheNumshardbits(value int) {
	C.rocksdb_options_set_table_cache_numshardbits(self.c, C.int(value))
}

// During data eviction of table's LRU cache, it would be inefficient
// to strictly follow LRU because this piece of memory will not really
// be released unless its refcount falls to zero. Instead, make two
// passes: the first pass will release items with refcount = 1,
// and if not enough space releases after scanning the number of
// elements specified by this parameter, we will remove items in LRU order.
// Default: 16
func (self *Options) SetTableCacheRemoveScanCountLimit(value int) {
	C.rocksdb_options_set_table_cache_remove_scan_count_limit(self.c, C.int(value))
}

// Size of one block in arena memory allocation.
// If <= 0, a proper value is automatically calculated (usually 1/10 of
// writer_buffer_size).
// Default: 0
func (self *Options) SetArenaBlockSize(value int) {
	C.rocksdb_options_set_arena_block_size(self.c, C.size_t(value))
}

// Disable automatic compactions. Manual compactions can still
// be issued on this database.
// Default: false
func (self *Options) SetDisableAutoCompactions(value bool) {
	C.rocksdb_options_set_disable_auto_compactions(self.c, C.int(btoi(value)))
}

// The following two options affect how archived logs will be deleted.
// 1. If both set to 0, logs will be deleted asap and will not get into
//    the archive.
// 2. If wal_ttl_seconds is 0 and wal_size_limit_mb is not 0,
//    WAL files will be checked every 10 min and if total size is greater
//    then wal_size_limit_mb, they will be deleted starting with the
//    earliest until size_limit is met. All empty files will be deleted.
// 3. If wal_ttl_seconds is not 0 and wall_size_limit_mb is 0, then
//    WAL files will be checked every wal_ttl_seconds / 2 and those that
//    are older than wal_ttl_seconds will be deleted.
// 4. If both are not 0, WAL files will be checked every 10 min and both
//    checks will be performed with ttl being first.
// Default: 0
func (self *Options) SetWALTtlSeconds(value uint64) {
	C.rocksdb_options_set_WAL_ttl_seconds(self.c, C.uint64_t(value))
}

// If total size of WAL files is greater then wal_size_limit_mb,
// they will be deleted starting with the earliest until size_limit is met
// Default: 0
func (self *Options) SetWalSizeLimitMb(value uint64) {
	C.rocksdb_options_set_WAL_size_limit_MB(self.c, C.uint64_t(value))
}

// Number of bytes to preallocate (via fallocate) the manifest files.
// Default is 4mb, which is reasonable to reduce random IO
// as well as prevent overallocation for mounts that preallocate
// large amounts of data (such as xfs's allocsize option).
// Default: 4mb
func (self *Options) SetManifestPreallocationSize(value int) {
	C.rocksdb_options_set_manifest_preallocation_size(self.c, C.size_t(value))
}

// Purge duplicate/deleted keys when a memtable is flushed to storage.
// Default: true
func (self *Options) SetPurgeRedundantKvsWhileFlush(value bool) {
	C.rocksdb_options_set_purge_redundant_kvs_while_flush(self.c, boolToChar(value))
}

// Data being read from file storage may be buffered in the OS
// Default: true
func (self *Options) SetAllowOsBuffer(value bool) {
	C.rocksdb_options_set_allow_os_buffer(self.c, boolToChar(value))
}

// Allow the OS to mmap file for reading sst tables.
// Default: false
func (self *Options) SetAllowMmapReads(value bool) {
	C.rocksdb_options_set_allow_mmap_reads(self.c, boolToChar(value))
}

// Allow the OS to mmap file for writing.
// Default: true
func (self *Options) SetAllowMmapWrites(value bool) {
	C.rocksdb_options_set_allow_mmap_writes(self.c, boolToChar(value))
}

// Disable child process inherit open files.
// Default: true
func (self *Options) SetIsFdCloseOnExec(value bool) {
	C.rocksdb_options_set_is_fd_close_on_exec(self.c, boolToChar(value))
}

// Skip log corruption error on recovery (If client is ok with
// losing most recent changes)
// Default: false
func (self *Options) SetSkipLogErrorOnRecovery(value bool) {
	C.rocksdb_options_set_skip_log_error_on_recovery(self.c, boolToChar(value))
}

// If not zero, dump stats to LOG every stats_dump_period_sec
// Default: 3600 (1 hour)
func (self *Options) SetStatsDumpPeriodSec(value uint) {
	C.rocksdb_options_set_stats_dump_period_sec(self.c, C.uint(value))
}

// If set true, will hint the underlying file system that the file
// access pattern is random, when a sst file is opened.
// Default: true
func (self *Options) SetAdviseRandomOnOpen(value bool) {
	C.rocksdb_options_set_advise_random_on_open(self.c, boolToChar(value))
}

// Specify the file access pattern once a compaction is started.
// It will be applied to all input files of a compaction.
// Default: NormalCompactionAccessPattern
func (self *Options) SetAccessHintOnCompactionStart(value CompactionAccessPattern) {
	C.rocksdb_options_set_access_hint_on_compaction_start(self.c, C.int(value))
}

// Use adaptive mutex, which spins in the user space before resorting
// to kernel. This could reduce context switch when the mutex is not
// heavily contended. However, if the mutex is hot, we could end up
// wasting spin time.
// Default: false
func (self *Options) SetUseAdaptiveMutex(value bool) {
	C.rocksdb_options_set_use_adaptive_mutex(self.c, boolToChar(value))
}

// Allows OS to incrementally sync files to disk while they are being
// written, asynchronously, in the background.
// Issue one request for every bytes_per_sync written.
// Default: 0 (disabled)
func (self *Options) SetBytesPerSync(value uint64) {
	C.rocksdb_options_set_bytes_per_sync(self.c, C.uint64_t(value))
}

// The compaction style.
// Default: LevelCompactionStyle
func (self *Options) SetCompactionStyle(value CompactionStyle) {
	C.rocksdb_options_set_compaction_style(self.c, C.int(value))
}

// The options needed to support Universal Style compactions.
// Default: nil
func (self *Options) SetUniversalCompactionOptions(value *UniversalCompactionOptions) {
	C.rocksdb_options_set_universal_compaction_options(self.c, value.c)
}

// The options for FIFO compaction style
// Default: nil
func (self *Options) SetFIFOCompactionOptions(value *FIFOCompactionOptions) {
	C.rocksdb_options_set_fifo_compaction_options(self.c, value.c)
}

// If true, compaction will verify checksum on every read that happens
// as part of compaction
// Default: true
func (self *Options) SetVerifyChecksumsInCompaction(value bool) {
	C.rocksdb_options_set_verify_checksums_in_compaction(self.c, boolToChar(value))
}

// Use KeyMayExist API to filter deletes when this is true.
// If KeyMayExist returns false, i.e. the key definitely does not exist, then
// the delete is a noop. KeyMayExist only incurs in-memory look up.
// This optimization avoids writing the delete to storage when appropriate.
// Default: false
func (self *Options) SetFilterDeletes(value bool) {
	C.rocksdb_options_set_filter_deletes(self.c, boolToChar(value))
}

// An iteration->Next() sequentially skips over keys with the same
// user-key unless this option is set. This number specifies the number
// of keys (with the same userkey) that will be sequentially
// skipped before a reseek is issued.
// Default: 8
func (self *Options) SetMaxSequentialSkipInIterations(value uint64) {
	C.rocksdb_options_set_max_sequential_skip_in_iterations(self.c, C.uint64_t(value))
}

// Allows thread-safe inplace updates. Requires Updates iff
// * key exists in current memtable
// * new sizeof(new_value) <= sizeof(old_value)
// * old_value for that key is a put i.e. kTypeValue
// Default: false.
func (self *Options) SetInplaceUpdateSupport(value bool) {
	C.rocksdb_options_set_inplace_update_support(self.c, boolToChar(value))
}

// Number of locks used for inplace update.
// Default: 10000, if inplace_update_support = true, else 0.
func (self *Options) SetInplaceUpdateNumLocks(value int) {
	C.rocksdb_options_set_inplace_update_num_locks(self.c, C.size_t(value))
}

// If prefix_extractor is set and bloom_bits is not 0, create prefix bloom
// for memtable.
// Default: 0
func (self *Options) SetMemtablePrefixBloomBits(value uint32) {
	C.rocksdb_options_set_memtable_prefix_bloom_bits(self.c, C.uint32_t(value))
}

// Number of hash probes per key.
// Default: 6
func (self *Options) SetMemtablePrefixBloomProbes(value uint32) {
	C.rocksdb_options_set_memtable_prefix_bloom_probes(self.c, C.uint32_t(value))
}

// Control locality of bloom filter probes to improve cache miss rate.
// This option only applies to memtable prefix bloom and plaintable
// prefix bloom. It essentially limits the max number of cache lines each
// bloom filter check can touch.
// This optimization is turned off when set to 0. The number should never
// be greater than number of probes. This option can boost performance
// for in-memory workload but should use with care since it can cause
// higher false positive rate.
// Default: 0
func (self *Options) SetBloomLocality(value uint32) {
	C.rocksdb_options_set_bloom_locality(self.c, C.uint32_t(value))
}

// Maximum number of successive merge operations on a key in the memtable.
//
// When a merge operation is added to the memtable and the maximum number of
// successive merges is reached, the value of the key will be calculated and
// inserted into the memtable instead of the merge operation. This will
// ensure that there are never more than max_successive_merges merge
// operations in the memtable.
//
// Default: 0 (disabled)
func (self *Options) SetMaxSuccessiveMerges(value int) {
	C.rocksdb_options_set_max_successive_merges(self.c, C.size_t(value))
}

// The number of partial merge operands to accumulate before partial
// merge will be performed. Partial merge will not be called
// if the list of values to merge is less than min_partial_merge_operands.
//
// If min_partial_merge_operands < 2, then it will be treated as 2.
// Default: 2
func (self *Options) SetMinPartialMergeOperands(value uint32) {
	C.rocksdb_options_set_min_partial_merge_operands(self.c, C.uint32_t(value))
}

// Allow RocksDB to use thread local storage to optimize performance.
// Default: true
// This was removed in version 3.6 - see:
// https://github.com/facebook/rocksdb/blob/master/HISTORY.md#public-api-changes-4
//func (self *Options) SetAllowThreadLocal(value bool) {
//	C.rocksdb_options_set_allow_thread_local(self.c, boolToChar(value))
//}

// If enabled, then we should collect metrics about database operations.
// Default: false
func (self *Options) EnableStatistics() {
	C.rocksdb_options_enable_statistics(self.c)
}

// Set appropriate parameters for bulk loading.
//
// All data will be in level 0 without any automatic compaction.
// It's recommended to manually call CompactRange(NULL, NULL) before reading
// from the database, because otherwise the read can be very slow.
func (self *Options) PrepareForBulkLoad() {
	C.rocksdb_options_prepare_for_bulk_load(self.c)
}

// SetMemtableVectorRep sets a MemTableRep which is backed by a vector.
//
// On iteration, the vector is sorted. This is useful for workloads where
// iteration is very rare and writes are generally not issued after reads begin.
func (self *Options) SetMemtableVectorRep() {
	C.rocksdb_options_set_memtable_vector_rep(self.c)
}

// SetHashSkipListRep sets a hash skip list as MemTableRep.
//
// It contains a fixed array of buckets, each
// pointing to a skiplist (null if the bucket is empty).
//
// bucketCount:             number of fixed array buckets
// skiplistHeight:          the max height of the skiplist
// skiplistBranchingFactor: probabilistic size ratio between adjacent
//                          link lists in the skiplist
func (self *Options) SetHashSkipListRep(bucketCount int, skiplistHeight, skiplistBranchingFactor int32) {
	C.rocksdb_options_set_hash_skip_list_rep(self.c, C.size_t(bucketCount), C.int32_t(skiplistHeight), C.int32_t(skiplistBranchingFactor))
}

// SetHashLinkListRep sets a hashed linked list as MemTableRep.
//
// It contains a fixed array of buckets, each pointing to a sorted single
// linked list (null if the bucket is empty).
//
// bucketCount: number of fixed array buckets
func (self *Options) SetHashLinkListRep(bucketCount int) {
	C.rocksdb_options_set_hash_link_list_rep(self.c, C.size_t(bucketCount))
}

// SetPlainTableFactory sets a plain table factory with prefix-only seek.
//
// For this factory, you need to set prefix_extractor properly to make it
// work. Look-up will starts with prefix hash lookup for key prefix. Inside the
// hash bucket found, a binary search is executed for hash conflicts. Finally,
// a linear search is used.
//
// keyLen: 			plain table has optimization for fix-sized keys,
// 					which can be specified via keyLen.
// bloomBitsPerKey: the number of bits used for bloom filer per prefix. You
//                  may disable it by passing a zero.
// hashTableRatio:  the desired utilization of the hash table used for prefix
//                  hashing. hashTableRatio = number of prefixes / #buckets
//                  in the hash table
// indexSparseness: inside each prefix, need to build one index record for how
//                  many keys for binary search inside each hash bucket.
func (self *Options) SetPlainTableFactory(keyLen uint32, bloomBitsPerKey int, hashTableRatio float64, indexSparseness int) {
	C.rocksdb_options_set_plain_table_factory(self.c, C.uint32_t(keyLen), C.int(bloomBitsPerKey), C.double(hashTableRatio), C.size_t(indexSparseness))
}

func (self *Options) SetCreateIfMissingColumnFamilies(value bool) {
	C.rocksdb_options_set_create_missing_column_families(self.c, boolToChar(value))
}

// Destroy deallocates the Options object.
func (self *Options) Destroy() {
	C.rocksdb_options_destroy(self.c)
	if self.ccmp != nil {
		C.rocksdb_comparator_destroy(self.ccmp)
	}
	if self.cmo != nil {
		C.rocksdb_mergeoperator_destroy(self.cmo)
	}
	if self.cst != nil {
		C.rocksdb_slicetransform_destroy(self.cst)
	}
	if self.ccf != nil {
		C.rocksdb_compactionfilter_destroy(self.ccf)
	}
	self.c = nil
	self.cmp = nil
	self.mo = nil
	self.env = nil
	self.st = nil
	self.cf = nil
	self.bbto = nil
}

func (self *Options) SetBlockBasedTableFactory(value *BlockBasedTableOptions) {
	self.bbto = value
	C.rocksdb_options_set_block_based_table_factory(self.c, value.c)
}
