#!/bin/bash
if [ "$1" = "false" ] && [ "$2" != "hyperledger" ]; then
rm -rf $HOME/gopath/src/github.com/hyperledger/
echo "Deleted hyperledger folder"
cp -r $HOME/gopath/src/github.com/$2 $HOME/gopath/src/github.com/hyperledger
echo "Copied User Directory into hyperledger"
elif [ "$2" != "hyperledger" ]; then
mv $HOME/gopath/src/github.com/$2 $HOME/gopath/src/github.com/hyperledger
fi
