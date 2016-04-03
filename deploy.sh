#!/bin/bash
if grep -q 'FAIL' "/$HOME/gopath/src/github.com/hyperledger/fabric/build-result.txt"; then
echo "Build Failed"
else
echo "Build passed"
fi
