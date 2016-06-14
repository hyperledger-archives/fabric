#! /bin/bash
# Ubuntu Server 14.04
# Require 15GB strage
# 
# Filesystem      Size  Used Avail Use% Mounted on
# udev             15G   12K   15G   1% /dev
# tmpfs           3.0G  364K  3.0G   1% /run
# /dev/xvda1       25G   14G   11G  58% /

# Test environment
# AWS EC2 Ubuntu Server 14.04 LTS (HVM)
# Storage: 25GB
# Instance Type: c4.4xlarge

# Foundation
sudo apt-get update
sudo apt-get -y upgrade
sudo apt-get -y install git
sudo apt-get -y install gcc
sudo apt-get -y install g++
sudo apt-get -y install make
sudo apt-get -y install unzip

# Add required library
sudo apt-get -y install zlib1g-dev
sudo apt-get -y install libsnappy-dev
sudo apt-get -y install libbz2-dev

# Install Golang 1.6.2
cd /tmp
wget https://storage.googleapis.com/golang/go1.6.2.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.6.2.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >>  $HOME/.bash_profile

mkdir $HOME/go
mkdir $HOME/work

echo 'export GOROOT=$HOME/go' >> $HOME/.profile
echo 'export PATH=$PATH:$GOROOT/bin' >>  ~/.profile
echo 'export GOPATH=$HOME/work' >> $HOME/.bash_profile
. $HOME/.bash_profile

go version

# Use for make rocksdb
# Prevent "virtual memory exhausted: Cannot allocate memory"
sudo dd if=/dev/zero of=/swapfile bs=1024 count=256k
sudo mkswap /swapfile
sudo swapon /swapfile
sudo sh -c 'echo "/swapfile       none    swap    sw      0       0 " >> /etc/fstab'
echo 10 | sudo tee /proc/sys/vm/swappiness
echo vm.swappiness = 10 | sudo tee -a /etc/sysctl.conf
sudo chown root:root /swapfile 
sudo chmod 0600 /swapfile 

# Install rocksdb
cd /tmp
# master is bad. ref: https://github.com/tecbot/gorocksdb/issues/50
# git clone https://github.com/facebook/rocksdb.git
# cd rocksdb
wget https://github.com/facebook/rocksdb/archive/v4.5.1.tar.gz
tar xzvf v4.5.1.tar.gz
cd rocksdb-4.5.1
make
sudo make install

# Docker
sudo apt-get install apt-transport-https ca-certificates  
  
sudo apt-key adv --keyserver hkp://p80.pool.sks-keyservers.net:80 --recv-keys 58118E89F3A912897C070ADBF76221572C52609D
  
sudo sh -c "echo 'deb https://apt.dockerproject.org/repo ubuntu-trusty main' >> /etc/apt/sources.list.d/docker.list"
sudo apt-get update
sudo apt-get install -y linux-image-extra-$(uname -r)
sudo apt-get purge lxc-docker
sudo apt-cache policy docker-engine
sudo apt-get -y install apparmor
sudo apt-get -y install docker-engine
sudo service docker stop

# docker unuse sudo
sudo gpasswd -a $USER docker

# Prevent https://github.com/docker/docker/issues/18180
sudo service docker stop
sudo docker daemon --storage-driver=devicemapper &
sudo docker run hello-world

# Fabric
mkdir $GOPATH/src
cd $GOPATH/src
mkdir -p github.com/hyperledger
cd github.com/hyperledger
git clone https://github.com/hyperledger/fabric.git
cd $GOPATH/src/github.com/hyperledger/fabric

sudo mkdir /var/hyperledger
sudo chown -R $USER:$USER /var/hyperledger

exec sg docker "make peer"

