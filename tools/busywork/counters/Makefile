# Makefile for counters

############################################################################
# Documentation
############################################################################

# Note: You can easily maintain your own set of make targets by creating a
# file called 'private.mk' in this directory, and then tracking that file in
# your private fork. If 'private.mk' exists in this directory it will be
# included by this Makefile.

# The following are the supported make targets:

ifeq (make, targets)

############################################################################
# Stress tests

stress1

    A single validating peer with NOOPS consensus and no security. 64 clients
    each drive 1000 transactions into a single array without interlock.

stress2 stress2b stress2c stress2s

    NB: We are currently trying to work around issues in this test case, it is
    not being run exactly as described below, and does not run successfully
    with every consensus mode.

    A Docker-compose network of 4 validating peers running PBFT without
    security. Send 10,000 transactions into a single array as a single burst,
    using only the first peer, with one interlock at the end of the
    burst. Individual PBFT protocols are either (b)atch, (c)lassic or (s)ieve.

secure1

    A 4-node setup with security. Security-mode is so slow that this is not a
    very stressful test. Currently Sieve is the only PBFT protocol that works,
    so that's what we use.


############################################################################
# Miscellaneous

build

    Build the chaincode

endif

############################################################################
# Make targets
############################################################################

############################################################################
# Stress tests

NETWORK = ../bin/userModeNetwork

.PHONY: stress1
stress1: 
	@$(NETWORK) -noops 1
	@./driver \
		-security \
		-clients 64 \
		-transactions 1000 \
		-peerBurst 1000 \
		-keepLog


# Note, fabric buffers are too small to do 10K TX, must do 100 X 100
# We're using all peers here, once buffers increase we can go back to 1
# This will eventually be more stressful

.PHONY: stress2 stress2b stress2c stress2s
STRESS2 = ./driver \
		-transactions 10000 \
		-peerBurst 100 \
		-interlock \
		-keepLog \
		-checkAgreement -dupCheck

stress2b: 
	@$(NETWORK) -batch 4
	@$(STRESS2)

stress2c: 
	@$(NETWORK) -classic 4
	@$(STRESS2)

stress2s: 
	@$(NETWORK) -sieve 4
	@$(STRESS2)

stress2: stress2b stress2c stress2s


.PHONY: secure1
secure1:
	@$(NETWORK) -sieve -security 4
	@./driver \
		-transactions 100 \
		-peerBurst 10 \
		-interlock \
		-keepLog \
		-checkAgreement -dupCheck


############################################################################
# Miscellaneous

.PHONY: build
build:
	go build -a

# Load the user's private makefile, if it exists.

ifneq (,$(wildcard private.mk))
include private.mk
endif
