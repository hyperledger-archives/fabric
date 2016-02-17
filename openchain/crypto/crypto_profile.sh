#!/usr/bin/env bash

go test -c
#./crypto.test -test.cpuprofile=crypto.prof
#./crypto.test -test.bench BenchmarkConfidentialTCertHExecuteTransaction -test.run XXX -test.cpuprofile=crypto.prof
./crypto.test -test.bench BenchmarkSign -test.run XXX -test.cpuprofile=crypto.prof
go tool pprof crypto.test crypto.prof