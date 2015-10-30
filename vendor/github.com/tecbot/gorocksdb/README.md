# gorocksdb, a Go wrapper for RocksDB

[![Build Status](https://travis-ci.org/tecbot/gorocksdb.png)](https://travis-ci.org/tecbot/gorocksdb) [![GoDoc](https://godoc.org/github.com/tecbot/gorocksdb?status.png)](http://godoc.org/github.com/tecbot/gorocksdb)

## Building

You'll need the shared library build of
[RocksDB](https://github.com/facebook/rocksdb) installed on your machine, simply run:

    make shared_lib

Now, if you build RocksDB you can install gorocksdb:

    CGO_CFLAGS="-I/path/to/rocksdb/include" \
    CGO_LDFLAGS="-L/path/to/rocksdb -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy" \
      go get github.com/tecbot/gorocksdb
