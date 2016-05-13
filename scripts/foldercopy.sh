#!/bin/bash

if [ "$1" = "false" ] && [ "$2" != "hyperledger" ]; then
	
	echo " Pull Request number is $1 "
	echo " User Name is $2 "
	echo " Repository Name is $3 "

rm -rf $HOME/gopath/src/github.com/hyperledger/
	
	echo "creating fabric folder to copy files"

mkdir -p $HOME/gopath/src/github.com/hyperledger/fabric

	echo "hyperledger/fabric folder created"

cp -r $HOME/gopath/src/github.com/$2/$3/* $HOME/gopath/src/github.com/hyperledger/fabric/

	echo "Copied $2 files into hyperledger/fabric folder"

elif [ "$2" != "hyperledger" ]; then

mkdir -p $HOME/gopath/src/github.com/hyperledger/fabric

	echo "hyperledger/fabric folder created"

cp -r $HOME/gopath/src/github.com/$2/$3/* $HOME/gopath/src/github.com/hyperledger/fabric/
	
	echo "copied $2 user repo into hyperledger/fabric folder"

fi
