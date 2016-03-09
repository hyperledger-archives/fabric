#!/bin/bash
echo "entering"
echo " Pull value $1"
if [ "$1" = "false" ]; then
rm -rf $HOME/gopath/src/github.com/openblockchain/
echo "deleted"
cp -r $HOME/gopath/src/github.com/rameshthoomu $HOME/gopath/src/github.com/openblockchain
echo "copied"
fi
echo "Completed"
