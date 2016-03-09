#!/bin/bash
if [ "$TRAVIS_PULL_REQUEST" -eq 'false' ]; then
rm -rf $HOME/gopath/src/github.com/openblockchain/
echo "deleted"
cp -r $HOME/gopath/src/github.com/rameshthoomu $HOME/gopath/src/github.com/openblockchain
echo "copied"
fi
