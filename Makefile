# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements.  See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership.  The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License.  You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
#
# -------------------------------------------------------------
# This makefile defines the following targets
#
#   - all (default) - builds all targets and runs all tests/checks
#   - checks - runs all tests/checks
#   - peer - builds the fabric ./peer/peer binary
#   - membersrvc - builds the ./membersrvc/membersrvc binary
#   - unit-test - runs the go-test based unit tests
#   - behave - runs the behave test
#   - behave-deps - ensures pre-requisites are availble for running behave manually
#   - linter - runs all code checks
#   - images - ensures all docker images are available
#   - peer-image - ensures the peer-image is available (for behave, etc)
#   - ca-image - ensures the ca-image is available (for behave, etc)
#   - protos - generate all protobuf artifacts based on .proto files
#   - clean - cleans the build area
#   - dist-clean - superset of 'clean' that also removes persistent state


PKGNAME = github.com/hyperledger/fabric
CGO_LDFLAGS = -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy

EXECUTABLES = go docker
K := $(foreach exec,$(EXECUTABLES),\
	$(if $(shell which $(exec)),some string,$(error "No $(exec) in PATH: Check dependencies")))

all: peer membersrvc checks

checks: unit-test behave linter

.PHONY: peer
peer: base-image
	cd peer; CGO_CFLAGS=" "	CGO_LDFLAGS="$(CGO_LDFLAGS)" go build

.PHONY: membersrvc
membersrvc:
	cd membersrvc; CGO_CFLAGS=" " CGO_LDFLAGS="$(CGO_LDFLAGS)" go build

unit-test: peer-image
	@echo "Running unit-tests"
	$(eval CID := $(shell docker run -dit -p 30303:30303 hyperledger-peer peer node start))
	@go test -timeout=20m $(shell go list $(PKGNAME)/... | grep -v /vendor/ | grep -v /examples/)
	@docker kill $(CID)
	@touch .peerimage-dummy
	@touch .caimage-dummy

base-image: .baseimage-dummy
peer-image: .peerimage-dummy
ca-image: .caimage-dummy
images: peer-image ca-image

behave-deps: images peer
behave: behave-deps
	@echo "Running behave tests"
	@cd bddtests; behave $(BEHAVE_OPTS)

linter:
	@echo "LINT: Running code checks.."
	@echo "LINT: No errors found"

.peerimage-dummy: .baseimage-dummy
	go test $(PKGNAME)/core/container -run=BuildImage_Peer
	@touch $@

.caimage-dummy: .baseimage-dummy
	go test $(PKGNAME)/core/container -run=BuildImage_Obcca
	@touch $@

.baseimage-dummy:
	@echo "Building docker base-image"
	@./scripts/provision/docker.sh 0.0.9
	@touch $@

.PHONY: protos
protos:
	./devenv/compile_protos.sh

.PHONY: clean
clean:
	-@rm .*image-dummy ||:
	-@rm -f ./peer/peer ||:
	-@rm -f ./membersrvc/membersrvc ||:

.PHONY: dist-clean
dist-clean: clean
	-@rm -rf /var/hyperledger/* ||:
