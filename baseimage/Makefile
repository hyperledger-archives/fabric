NAME="hyperledger/fabric-baseimage"
VERSION=0.0.9
ARCH=$(shell uname -m)
DOCKER_NAME ?= hyperledger/fabric-baseimage
DOCKER_TAG ?= $(ARCH)-$(VERSION)
VAGRANTIMAGE=packer_virtualbox-iso_virtualbox.box

DOCKER_BASE_x86_64=ubuntu:trusty

DOCKER_BASE=$(DOCKER_BASE_$(ARCH))

ifeq ($(DOCKER_BASE), )
$(error "Architecture \"$(ARCH)\" is unsupported")
endif


# strips off the post-processors that try to upload artifacts to the cloud
packer-local.json: packer.json
	jq 'del(."post-processors"[0][1]) | del(."post-processors"[1][1])' packer.json > $@

all: vagrant docker

$(VAGRANTIMAGE): packer-local.json
	BASEIMAGE_RELEASE=$(VERSION) \
	packer build -only virtualbox-iso packer-local.json

Dockerfile:

docker: Dockerfile.in Makefile
	@echo "Generating docker"
	@cat Dockerfile.in | \
	sed -e  "s|_DOCKER_BASE_|$(DOCKER_BASE)|" > Dockerfile
	docker build -t $(DOCKER_NAME):$(DOCKER_TAG) .


vagrant: $(VAGRANTIMAGE) remove
	vagrant box add -name $(NAME) $(VAGRANTIMAGE)

push:
	@echo "You will need your ATLAS_TOKEN set for this to succeed"
	packer push -name $(NAME) packer.json

remove:
	-vagrant box remove --box-version 0 $(NAME)

clean: remove
	-rm $(VAGRANTIMAGE)
	-rm Dockerfile
	-rm packer-local.json
