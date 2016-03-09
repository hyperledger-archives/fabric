#!/bin/bash
echo "entering"
echo " Pull value $TR_PULL_REQUEST"
if [ "$TR_PULL_REQUEST" = "false" ]; then
rm -rf $HOME/gopath/src/github.com/openblockchain/
echo "deleted"
cp -r $HOME/gopath/src/github.com/rameshthoomu $HOME/gopath/src/github.com/openblockchain
echo "copied"
fi
echo "Completed"
