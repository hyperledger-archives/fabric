#!/bin/bash
echo " Pull Request value is $1"
if [ "$1" = "false" ] && [ "$2" != "openblockchain" ]; then
rm -rf $HOME/gopath/src/github.com/openblockchain/
echo "Deleted openblockchain folder"
cp -r $HOME/gopath/src/github.com/$2 $HOME/gopath/src/github.com/openblockchain
echo "Copied User Directory into openblockchain"
elif [ "$2" != "openblockchain" ]; then
echo "creatng mkdir"
#mkdir -p $HOME/gopath/src/github.com/openblockchain/
mv $HOME/gopath/src/github.com/$2 $HOME/gopath/src/github.com/openblockchain
echo $HOME/gopath/src/github.com/openblockchain
ls $HOME/gopath/src/github.com/openblockchain/obc-peer
echo "Created and copied openblockchain folder"
fi
