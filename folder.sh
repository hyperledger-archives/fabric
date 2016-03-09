#!/bin/bash
echo "entering"
echo $TRAVIS_PULL_REQUEST
if [ "$TRAVIS_PULL_REQUEST" = "false" ]; then
rm -rf $HOME/gopath/src/github.com/openblockchain/
echo "deleted"
cp -r $HOME/gopath/src/github.com/rameshthoomu $HOME/gopath/src/github.com/openblockchain
echo "copied"
fi
