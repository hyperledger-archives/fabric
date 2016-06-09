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
#   - gotools - installs go tools like golint
#   - linter - runs all code checks
#   - images - ensures all docker images are available
#   - peer-image - ensures the peer-image is available (for behave, etc)
#   - ca-image - ensures the ca-image is available (for behave, etc)
#   - protos - generate all protobuf artifacts based on .proto files
#   - node-sdk - builds the node.js client-sdk
#   - clean - cleans the build area
#   - dist-clean - superset of 'clean' that also removes persistent state


PKGNAME = github.com/hyperledger/fabric
CGO_LDFLAGS = -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy

EXECUTABLES = go docker git
K := $(foreach exec,$(EXECUTABLES),\
	$(if $(shell which $(exec)),some string,$(error "No $(exec) in PATH: Check dependencies")))

# Make our baseimage depend on any changes to images/base or scripts/provision
BASEIMAGE_RELEASE = $(shell cat ./images/base/release)
BASEIMAGE_DEPS    = $(shell git ls-files images/base scripts/provision)

GOTOOLS = golint govendor goimports protoc-gen-go
GOTOOLS_BIN = $(patsubst %,$(GOPATH)/bin/%, $(GOTOOLS))

# go tool->path mapping
go.fqp.govendor  := github.com/kardianos/govendor
go.fqp.golint    := github.com/golang/lint/golint
go.fqp.goimports := golang.org/x/tools/cmd/goimports

all: peer membersrvc checks

checks: unit-test behave linter

.PHONY: peer
peer: base-image
	cd peer; CGO_CFLAGS=" "	CGO_LDFLAGS="$(CGO_LDFLAGS)" go build

.PHONY: membersrvc
membersrvc:
	cd membersrvc; CGO_CFLAGS=" " CGO_LDFLAGS="$(CGO_LDFLAGS)" go build

unit-test: peer-image gotools
	@./scripts/goUnitTests.sh
	@touch .peerimage-dummy
	@touch .caimage-dummy

base-image: .baseimage-dummy
peer-image: .peerimage-dummy
ca-image: .caimage-dummy

.PHONY: images
images: peer-image ca-image

behave-deps: images peer
behave: behave-deps
	@echo "Running behave tests"
	@cd bddtests; behave $(BEHAVE_OPTS)

gotools: $(GOTOOLS_BIN)

linter: gotools
	@echo "LINT: Running code checks.."
	@echo "LINT: No errors found"

.peerimage-dummy: .baseimage-dummy
	go test $(PKGNAME)/core/container -run=BuildImage_Peer
	@touch $@

.caimage-dummy: .baseimage-dummy
	go test $(PKGNAME)/core/container -run=BuildImage_Obcca
	@touch $@

.baseimage-dummy: $(BASEIMAGE_DEPS)
	@echo "Building docker base-image"
	@./scripts/provision/docker.sh $(BASEIMAGE_RELEASE)
	@touch $@

# Special override for protoc-gen-go since we want to use the version vendored with the project
gotool.protoc-gen-go:
	mkdir -p $(GOPATH)/src/github.com/golang/protobuf/
	cp -r $(GOPATH)/src/github.com/hyperledger/fabric/vendor/github.com/golang/protobuf/ $(GOPATH)/src/github.com/golang/
	go install github.com/golang/protobuf/protoc-gen-go
	rm -rf $(GOPATH)/src/github.com/golang/protobuf

# Default rule for gotools uses the name->path map for a generic 'go get' style build
gotool.%:
	$(eval TOOL = ${subst gotool.,,${@}})
	go get ${go.fqp.${TOOL}}

$(GOPATH)/bin/%:
	$(eval TOOL = ${subst $(GOPATH)/bin/,,${@}})
	$(MAKE) gotool.$(TOOL)

build/bin:
	mkdir -p $@

.PHONY: protos
protos:
	./devenv/compile_protos.sh

.PHONY: node-sdk
node-sdk:
	cp ./protos/*.proto ./sdk/node/lib/protos
	cp ./membersrvc/protos/*.proto ./sdk/node/lib/protos
	cd ./sdk/node && npm install && sudo npm install -g typescript && sudo npm install typings --global && typings install
	cd ./sdk/node && tsc
	cd ./sdk/node && ./makedoc.sh

.PHONY: clean
clean:
	-@rm -rf build ||:
	-@rm -f .*image-dummy ||:
	-@rm -f ./peer/peer ||:
	-@rm -f ./membersrvc/membersrvc ||:
	-@rm -f $(GOTOOLS_BIN) ||:

.PHONY: dist-clean
dist-clean: clean
	-@rm -rf /var/hyperledger/* ||:
