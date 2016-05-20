## Setting up the development environment

### Overview
The current supported development environment utilizes Vagrant running an Ubuntu image, which in turn launches Docker containers. Conceptually, the Host launches a VM, which in turn launches Docker containers.

**Host -> VM -> Docker**

This model allows developers to leverage their favorite OS/editors and execute the system in a controlled environment that is consistent amongst the development team.

- Note that your Host should not run within a VM. If you attempt this, the VM within your Host may fail to boot with a message indicating that VT-x is not available.

### Prerequisites
* [Git client](https://git-scm.com/downloads)
* [Go](https://golang.org/) - 1.6 or later
* [Vagrant](https://www.vagrantup.com/) - 1.7.4 or later
* [VirtualBox](https://www.virtualbox.org/) - 5.0 or later
* BIOS Enabled Virtualization - Varies based on hardware

Note: The BIOS Enabled Virtualization may be within the CPU or Security settings of the BIOS

### Steps

#### Set your GOPATH
Make sure you have properly setup your Host's [GOPATH environment variable](https://github.com/golang/go/wiki/GOPATH). This allows for both building within the Host and the VM.

#### Note to Windows users

If you are running Windows, before running any `git clone` commands, run the following command.
```
git config --get core.autocrlf
```
If `core.autocrlf` is set to `true`, you must set it to `false` by running
```
git config --global core.autocrlf false
```
If you continue with `core.autocrlf` set to `true`, the `vagrant up` command will fail with the error `./setup.sh: /bin/bash^M: bad interpreter: No such file or directory`

#### Cloning the Peer project

Create a fork of the [fabric](https://github.com/hyperledger/fabric) repository using the GitHub web interface. Next, clone your fork in the appropriate location.

```
cd $GOPATH/src
mkdir -p github.com/hyperledger
cd github.com/hyperledger
git clone https://github.com/<username>/fabric.git
```


#### Boostrapping the VM using Vagrant

```
cd $GOPATH/src/github.com/hyperledger/fabric/devenv
vagrant up
```

**NOTE:** If you intend to run the development environment behind an HTTP Proxy, you need to configure the guest so that the provisioning process may complete.  You can achieve this via the *vagrant-proxyconf* plugin. Install with *vagrant plugin install vagrant-proxyconf* and then set the VAGRANT_HTTP_PROXY and VAGRANT_HTTPS_PROXY environment variables *before* you execute *vagrant up*. More details are available here: https://github.com/tmatilai/vagrant-proxyconf/

**NOTE #2:** The first time you run this command it may take quite a while to complete (it could take 30 minutes or more depending on your environment) and at times it may look like it's not doing anything. As long you don't get any error messages just leave it alone, it's all good, it's just cranking.

Once complete, you should now be able to SSH into your new VM with the following command from the same directory.

    vagrant ssh

Once inside the VM, you can find the peer project under $GOPATH/src/github.com/hyperledger/fabric (as well as /hyperledger).

**NOTE:** any time you *git clone* any of the projects in your Host's fabric directory (under $GOPATH/src/github.com/hyperledger/fabric), the update will be instantly available within the VM fabric directory.

## Building outside of Vagrant
It is possible to build the project outside of Vagrant (e.g. when one already runs Linux and wants to develop natively). Generally speaking, one has to match the vagrant [setup file](https://github.com/hyperledger/fabric/blob/master/devenv/setup.sh).

### Prerequisites
* [Git client](https://git-scm.com/downloads)
* [Go](https://golang.org/) - 1.6 or later
* [RocksDB](https://github.com/facebook/rocksdb/blob/master/INSTALL.md) version 4.1 and its dependencies
* [Docker](https://docs.docker.com/engine/installation/)
* [Pip](https://pip.pypa.io/en/stable/installing/)
* Set the maximum number of open files to 10000 or greater for your OS

### Docker
Make sure that the Docker daemon initialization includes the options
```
-H tcp://0.0.0.0:2375 -H unix:///var/run/docker.sock
```

Typically, docker runs as a `service` task, with configuration file at `/etc/default/docker`.

Be aware that the Docker bridge (the `CORE_VM_ENDPOINT`) may not come
up at the IP address currently assumed by the test environment
(`172.17.0.1`). Use `ifconfig` or `ip addr` to find the docker bridge.

### Building RocksDB
```
apt-get install -y libsnappy-dev zlib1g-dev libbz2-dev
cd /tmp
git clone https://github.com/facebook/rocksdb.git
cd rocksdb
git checkout v4.1
PORTABLE=1 make shared_lib
INSTALL_PATH=/usr/local make install-shared
```

### `pip`, `behave` and `docker-compose`
```
pip install --upgrade pip
pip install behave nose docker-compose
pip install -I flask==0.10.1 python-dateutil==2.2 pytz==2014.3 pyyaml==3.10 couchdb==1.0 flask-cors==2.0.1 requests==2.4.3
```

### Building on Z
To make building on Z easier and faster, [this script](https://github.com/hyperledger/fabric/tree/master/devenv/setupRHELonZ.sh) is provided (which is similar to the [setup file](https://github.com/hyperledger/fabric/blob/master/devenv/setup.sh) provided for vagrant). This script has been tested only on RHEL 7.2 and has some assumptions one might want to re-visit (firewall settings, development as root user, etc.). It is however sufficient for development in a personally-assigned VM instance.

To get started, from a freshly installed OS:
```
sudo su
yum install git
mkdir -p $HOME/git/src/github.com/hyperledger
cd $HOME/git/src/github.com/hyperledger
git clone https://github.com/vpaprots/fabric.git
source fabric/devenv/setupRHELonZ.sh
```
From there, follow instructions at [Installation](install.md):

```
cd $GOPATH/src/github.com/hyperledger/fabric
make peer unit-test behave
```

