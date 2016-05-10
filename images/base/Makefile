NAME="hyperledger/fabric-baseimage"
VAGRANTIMAGE=packer_virtualbox-iso_virtualbox.box

# strips off the post-processors that try to upload artifacts to the cloud
packer-local.json: packer.json
	jq 'del(."post-processors"[0][1]) | del(."post-processors"[1][1])' packer.json > $@

all:
	packer build packer.json

$(VAGRANTIMAGE): packer-local.json
	packer build -only virtualbox-iso packer-local.json

docker: packer-local.json
	packer build -only docker packer-local.json

vagrant: $(VAGRANTIMAGE) remove
	vagrant box add -name $(NAME) $(VAGRANTIMAGE)

push:
	@echo "You will need your ATLAS_TOKEN set for this to succeed"
	packer push -name $(NAME) packer.json

remove:
	-vagrant box remove --box-version 0 $(NAME)

clean: remove
	-rm $(VAGRANTIMAGE)
	packer-local.json
