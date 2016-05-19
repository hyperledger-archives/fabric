# Makefile for crypto benchmarks, still in a nascent state...

GOARCH = $(shell go env GOARCH)

# These values were chosen because they take about 10 seconds to run on a
# modern server.

# Go's 'amd64' architecture has a hand-written assembler implementation of
# P256
ifeq ($(GOARCH),amd64)
P256_SIGNS    = 100000
P256_VERIFIES = 50000
else
P256_SIGNS    = 4000
P256_VERIFIES = 1500
endif

P384_SIGNS    = 300
P384_VERIFIES = 300


.PHONY: all
all:
	@go build
	@crypto P256_sign $(P256_SIGNS)
	@crypto P256_verify $(P256_VERIFIES)
	@crypto P384_sign $(P384_SIGNS)
	@crypto P384_verify $(P384_VERIFIES)
