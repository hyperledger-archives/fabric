PKGNAME = github.com/hyperledger/fabric
CGO_LDFLAGS = -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy

EXECUTABLES = go docker
K := $(foreach exec,$(EXECUTABLES),\
	$(if $(shell which $(exec)),some string,$(error "No $(exec) in PATH: Check dependencies")))

all: peer

peer: baseimage ./peer/peer

membersrvc: ./membersrvc/membersrvc

./peer/peer:
	cd peer; CGO_CFLAGS=" "	CGO_LDFLAGS="$(CGO_LDFLAGS)" go build

./membersrvc/membersrvc:
	cd membersrvc; CGO_CFLAGS=" " CGO_LDFLAGS="$(CGO_LDFLAGS)" go build

unit-test: baseimage
	@echo "Running unit-tests"
	@go test -timeout=20m $(shell go list $(PKGNAME)/... | grep -v /vendor/ | grep -v /examples/)
	@touch .peerimage-dummy

behave-deps: .peerimage-dummy

behave: behave-deps
	@echo "Running behave tests"
	@cd bddtests; behave

.peerimage-dummy: .baseimage-dummy
	go test $(PKGNAME)/core/container -run=BuildImage_Peer
	go test $(PKGNAME)/core/container -run=BuildImage_Obcca
	@touch $@

baseimage: .baseimage-dummy

.baseimage-dummy:
	@echo "Building docker base-image"
	@./scripts/provision/docker.sh 0.0.9
	@touch $@

protos:
	./devenv/compile_protos.sh

.PHONY: clean
clean:
	-@rm .baseimage-dummy ||:
	-@rm .peerimage-dummy ||:
	-@rm -f ./peer/peer ||:
	-@rm -f ./membersrvc/membersrvc ||:

.PHONY: dist-clean
dist-clean: clean
	-@rm -rf /var/hyperledger/* ||:
