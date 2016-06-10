The ".txt" files and "adummydir" are not used by chaincode_example02.go. 
They are in this folder just to test hash computation (see issue
https://github.com/hyperledger/fabric/issues/1789).

As long as these files don't change the chaincode ID should NOT change - ever.
And the hardcoded chaincode ID in chaincode_example04.go should work - always.
