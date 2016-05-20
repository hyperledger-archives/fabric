# Makefile for crypto benchmarks, still in a nascent state...

GOARCH = $(shell go env GOARCH)

# These values were chosen because they take about 10 seconds to run on a
# modern server. You can define TIMES on the make command line to define how
# many times each run is made. The default is 1 run; If TIMES > 1 then the
# results are averaged.

# Go's 'amd64' architecture has a hand-written assembler implementation of
# P256, SHA256 and SHA512
ifeq ($(GOARCH),amd64)
P256_SIGNS    = 100000
P256_VERIFIES = 50000
SHA256x8  = 20000000
SHA256x1K = 2000000
SHA256x8K = 200000
SHA512x8  = 20000000
SHA512x1K = 3000000
SHA512x8K = 400000
else
P256_SIGNS    = 4000
P256_VERIFIES = 1500
SHA256x8  = 5000000
SHA256x1K = 300000
SHA256x8K = 50000
SHA512x8  = 5000000
SHA512x1K = 300000
SHA512x8K = 50000
endif

P384_SIGNS    = 300
P384_VERIFIES = 300


.PHONY: all
all:
	@go build -a
	@benchmarks P256Sign   $(P256_SIGNS)    $(TIMES)
	@benchmarks P256Verify $(P256_VERIFIES) $(TIMES)
	@benchmarks P384Sign   $(P384_SIGNS)    $(TIMES)
	@benchmarks P384Verify $(P384_VERIFIES) $(TIMES)
	@benchmarks SHA256x8   $(SHA256x8)      $(TIMES)
	@benchmarks SHA256x1K  $(SHA256x1K)     $(TIMES)
	@benchmarks SHA256x8K  $(SHA256x8K)     $(TIMES)
	@benchmarks SHA512x8   $(SHA512x8)      $(TIMES)
	@benchmarks SHA512x1K  $(SHA512x1K)     $(TIMES)
	@benchmarks SHA512x8K  $(SHA512x8K)     $(TIMES)
