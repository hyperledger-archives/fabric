#!/bin/bash
if [ "$1" = "false" ] && [ "$2" != "hyperledger-incubator" ]; then
rm -rf $HOME/gopath/src/github.com/hyperledger-incubator/
echo "Deleted hyperledger-incubator folder"
cp -r $HOME/gopath/src/github.com/$2 $HOME/gopath/src/github.com/hyperledger-incubator
echo "Copied User Directory into hyperledger-incubator"
elif [ "$2" != "hyperledger-incubator" ]; then
mv $HOME/gopath/src/github.com/$2 $HOME/gopath/src/github.com/hyperledger-incubator
fi
