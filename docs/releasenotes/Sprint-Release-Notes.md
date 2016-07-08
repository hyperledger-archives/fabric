### Changes in the master branch effecting the external APIs or system behavior


##### 4/07/2016 commit 33bc136 changed chaincode interface
1. Deploy Transaction calls chaincode `Init` function
2. Invoke Transaction calls chaincode `Invoke` function
3. Query Transaction calls chaincode `Query` function
Please see [examples](https://github.com/hyperledger/fabric/tree/master/examples/chaincode/go) for more detail


##### 3/31/2016 POST /chaincode REST endpoint added

The /chaincode endpoint was added to replace the original /devops/deploy, /devops/invoke, and /devops/query endpoints, which are now deprecated. Please see the documentation here for further explanation and examples.

https://github.com/hyperledger/fabric/blob/master/docs/API/CoreAPI.md#chaincode


