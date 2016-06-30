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

stress2 stress2n stress2b

    NB: We are currently trying to work around issues in this test case, it is
    not being run exactly as described below, and does not run successfully
    with PBFT consensus.

    A Docker-compose network of 4 validating peers running PBFT without
    security. Send 10,000 transactions into a single array as a single burst,
    using only the first peer, with one interlock at the end of the
    burst. Consensus protocols are NOOPS (stress2n) or PBFT Batch (stress2b).

secure1

    A 4-node setup with security. Security-mode is so slow that this is not a
    very stressful test. This test uses PBFT batch mode.


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
		-clients 64 \
		-transactions 1000 \
		-peerBurst 1000 \
		-keepLog


# Note, fabric buffers are too small to do 10K TX, must do 100 X 100
# We're using all peers here, once buffers increase we can go back to 1
# This will eventually be more stressful

.PHONY: stress2 stress2n stress2b
STRESS2 = ./driver \
		-transactions 10000 \
		-peerBurst 100 \
		-interlock \
		-interlockTimeout 5m \
		-keepLog \
		-checkAgreement -dupCheck

STRESS2_CONFIG = \
	@CORE_PBFT_GENERAL_TIMEOUT_NULLREQUEST=3s

stress2n: 
	@$(STRESS2_CONFIG) \
	$(NETWORK) -noops 4
	@echo "makefile: Sleeping 10 seconds to allow peers to interlock"
	@sleep 10
	@$(STRESS2)

stress2b: 
	@$(STRESS2_CONFIG) \
	$(NETWORK) -batch 4
	@echo "makefile: Sleeping 10 seconds to allow peers to interlock"
	@sleep 10
	@$(STRESS2)

stress2: stress2n stress2b


.PHONY: secure1
secure1:
	@CORE_PBFT_GENERAL_TIMEOUT_NULLREQUEST=3s \
	$(NETWORK) -batch -security 4
	@echo "makefile: Sleeping 10 seconds to allow peers to interlock"
	@sleep 10	
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
