### dump_db_stats utility

This utility helps in analysing the hyperledger db contents; particularly, this utility prints
- the key-values which are over 1 MB (see const MaxValueSize, below),
- Further details about the key-values (e.g., number of transactions and over-sized transactions in the case of blockchain column family)
- LiveFilesMetaData about rocksdb .sst files
- Certain properties about rocksdb such as num-live-versions and cfstats (see 'struct Properties' at https://github.com/facebook/rocksdb/blob/master/include/rocksdb/db.h)

This utility can be run only on a off-line copy of the rocksdb i.e, the rocksdb instance that is not being used by a hyperledger peer currently.

Though, this utility does not modify the db contents in any manner but rocksdb library may run background activities
such as compaction and clearing write-ahead log files. In other words, you may observe that after running this utility,
files in the db are different from the initial state but the contents would still be same. **To avoid such side effects, it is reccommended that you make a copy of the hyperledger db and run this utility on the copy.**


### Running the utility
For running this utility, from directory $GOPATH/src/github.com/hyperledger/fabric/tools/dbstats - run command
<pre><code> go run dump_db_stats.go -dbDir 'path_to_db_dir' </code></pre>
 Note that the dbDir points to a directory that contains the dir named 'db'.
