# busywork

**busywork** is an exerciser framework for the
[Hyperledger fabric](https://github.com/hyperledger/fabric) project. As an
*exerciser*, **busywork** is not a real blockchain application, but is simply
a set of scripts, programs and utilities for stressing the blockchain fabric
in various, often randomized ways - hence the name. **busywork** applications
can be used both for correctness and performance testing, as well as for
simple benchmarks.

**busywork** is very much a work-in-progress. The effort to date has been
focussed on performance characterization, with correctness issues being a
natural fallout from stress testing.

## _**Use at Your Own Risk**_

As a system-level test environment, **busywork** assumes complete control over
Hyperledger fabric processes, containers and databases.  This environment has
been designed for a single user running in a VM or on a private system, or at
least a system where no other users are running the Hyperledger Fabric or
Docker containers. Outside of this type of environment, certain parts of
**busywork** may disrupt or destroy the work of others if used without
modification. _Caveat Emptor_.

## Prerequisites

If you are using the standard
[Vagrant environment](../../docs/dev-setup/devenv.md) then everything you need
has already been done. If you are running outside of Vagrant, some notes on
prerequisite packages can be found [here](prerequisites.md).

**busywork** scripts and procedures are written for users with password-less
`sudo` access, and `sudo`-less control of Docker. If you don't want to run as
**root**, consult your Linux distribution documentation for how to set up
password-less `sudo` access, and the Docker documentation for how to set up
`sudo`-less Docker access. The latter may be as simple as executing

    sudo usermod -a -G docker <username>
	

## What's Included?

* The [bin](bin/README.md) directory provides scripts and other support
used by all of the **busywork** applications, including numerous **make**
targets for setting up peer networks in various ways. You might find it useful
to put this directory on your **PATH**.

* The [busywork](busywork/README.md) directory defines a small Go package used
by **busywork** applications.

* The [benchmarks](benchmarks/README.md) directory contains the beginnings of
  a set of microbenchmarks for cryptographic primitives used by the
  Hyperledger fabric.

* The [tcl](tcl/README.md) directory defines Tcl packages.

The following applications (chaincodes) are currently provided. Each
application is documented separately.

* [counters](counters/README.md) is a chaincode that manages variable-sized
  arrays of counters. This provides for a simple but flexible self-checking
  test and benchmark environment. Make targets documented in the
  [Makefile](counters/Makefile) use a [driver](counters/driver) script to
  exercise the chaincode.

## BUSYWORK_HOME

**busywork** needs a well-known directory for log files and other
  configuration files that are generated dynamically. The **BUSYWORK_HOME**
  environment variable names this directory. If **BUSYWORK_HOME** is not
  defined, **busywork** scripts and applications create (if necessary) and use
  `~/.busywork` as the **BUSYWORK_HOME**. To avoid file name collisions, etc.,
  it is probably best to create a dedicated directory for **BUSYWORK_HOME** if
  you decide to define your own. The contents of **BUSYWORK_HOME** are
  described [here](bin/README.md#busywork-home).

## Getting Started

Change to the `hyperledger/fabric/tools/busywork/bin` directory and

    make build
	
to build the `peer` and `membersrvc` images. Then switch to the 
`busywork/counters` directory and 

    make stress1
	
to run a simple stress test. This stress test exercises a single peer
running `NOOPS` consensus.

Currently Sieve is the only PBFT consensus algorithm that is stable enough
to survive stress testing. The target

    make stress2s
	
runs Sieve on a 4-peer network. There is also a pre-canned target

    make secure1
	
that runs a 4-peer Sieve network with security.

The **busywork/counters/Makefile** allows you to define private targets in a
private makefile, **private.mk**, which is included by the main Makefile. For
example you might define a modification of the `stress2s` target in
**private.mk** to see what happens when the data arrays go from the default 1
counter (8 bytes) to 1000 counters (8000 bytes):

    .PHONY: stress2s1k
	stress2s1k:
            @$(NETWORK) -sieve 4
	        @$(STRESS2) -size 1000
	
