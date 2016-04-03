#!/bin/bash

helpme()
{
  cat <<HELPMEHELPME
Syntax: sudo $0

Installs the stuff needed to get the VirtualBox Ubuntu (or other similar Linux
host) into good shape to run our development environment.

This script needs to run as root.

The current directory must be the dev-env project directory.

HELPMEHELPME
}

if [[ "$1" == "-?" || "$1" == "-h" || "$1" == "--help" ]] ; then
  helpme
  exit 1
fi


set -e
set -x

# Setup the GOPATH; even though the shared folder spec gives the consul
# directory the right user/group, we need to set it properly on the
# parent path to allow subsequent "go get" commands to work. We can't do
# normal -R here because VMWare complains if we try to update the shared
# folder permissions, so we just update the folders that matter.
sudo mkdir -p $GOPATH
sudo mkdir -p $GOPATH/pkg
sudo mkdir -p $GOPATH/bin
sudo chown -R vagrant:vagrant $GOPATH
#find /opt/gopath -type d -maxdepth 3 | xargs sudo chown vagrant:vagrant
cat <<EOF >/tmp/gopath.sh
export GOPATH="$GOPATH"
export GOROOT="$GOROOT"
export PATH="$GOROOT/bin:$GOPATH/bin:\$PATH"
export CGO_CFLAGS=" "
export CGO_LDFLAGS="-lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy"
EOF
sudo mv /tmp/gopath.sh /etc/profile.d/gopath.sh
sudo chmod 0755 /etc/profile.d/gopath.sh
source /etc/profile.d/gopath.sh

# Install Go Vendor for vendor support
go get github.com/kardianos/govendor

# Install Golint and goimports
go get github.com/golang/lint/golint
go get golang.org/x/tools/cmd/goimports
