#!/bin/bash
if [ "$1" = "false" ] && [ "$2" != "openblockchain" ]; then
rm -rf $HOME/gopath/src/github.com/openblockchain/
echo "Deleted openblockchain folder"
cp -r $HOME/gopath/src/github.com/$2 $HOME/gopath/src/github.com/openblockchain
echo "Copied User Directory into openblockchain"
elif [ "$2" != "openblockchain" ]; then
mv $HOME/gopath/src/github.com/$2 $HOME/gopath/src/github.com/openblockchain
fi
