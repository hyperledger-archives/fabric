#!/bin/bash
if grep -q 'FAIL' "/$HOME/gopath/src/github.com/hyperledger/fabric/build-result.txt"; then
cat $HOME/gopath/src/github.com/hyperledger/fabric/build-result.txt
echo "=========> BUILD FAILED =========="
else
cat $HOME/gopath/src/github.com/hyperledger/fabric/build-result.txt
echo "==========> BUILD PASSED ========="
fi
