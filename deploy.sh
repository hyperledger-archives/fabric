#!/bin/bash
if grep -q 'FAIL' "/$HOME/gopath/src/github.com/openblockchain/obc-peer/build-result.txt"; then
echo "Build Failed"
else
echo "Build passed"
fi
