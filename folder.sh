#!/bin/bash
if [ "$TRAVIS_PULL_REQUEST" == "false" ]; then
rm -rf $HOME/gopath/src/github.com/openblockchain/
cp -r $HOME/gopath/src/github.com/rameshthoomu $HOME/gopath/src/github.com/openblockchain
fi
