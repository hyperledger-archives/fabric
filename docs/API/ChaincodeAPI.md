# Chaincode APIs

When the `Run` or `Query` function of a chaincode is invoked, a `stub *shim.ChaincodeStub` parameter is included. This stub can be used to call APIs that provide access to the ledger or other chaincodes. The following functions are currently available.

## State Access

`GetState(key string) ([]byte, error)` - Retrieves the value for the given key.

`PutState(key string, value []byte) error` - Stores the given key/value pair in the state. This will overwrite the existing value if a value is already present for the given key.

`DelState(key string) error` - Deletes the key and value associated with the key.

`RangeQueryState(startKey, endKey string) (*StateRangeQueryIterator, error)` - Retrieves an iterator for iterating over the key/value pairs between `startKey` and `endKey`, inclusive. While the iterator will return all keys lexically between the `startKey` and `endKey`, the keys will be returned in random order. The `Close` function of the iterator should be called when done to free resources.

## Access other chaincodes

It's possible for one deployed chaincode to call another deployed chaincode using the following APIs.

`InvokeChaincode(chaincodeName string, function string, args []string) ([]byte, error)` - Invokes the specified chaincode from the current chaincode.

`QueryChaincode(chaincodeName string, function string, args []string) ([]byte, error)` - Queries the specified chaincode from the current chaincode.

## Logging

Logging and logging control APIs are introduced below, and fully documented
with examples [here](../dev-setup/logging-control.md).

`SetLoggingLevel(LoggingLevel level)` - Control the logging level of the shim

`LogLevel(levelString string) (LoggingLevel, error)` - Convert a string to a `LoggingLevel`

`NewLogger(name string) *ChaincodeLogger` - Create a logging object for use by a chaincode

`(c *ChaincodeLogger) SetLevel(level LoggingLevel)` - Set the logging level of a chaincode logger

`(c *ChaincodeLogger) IsEnabledFor(level LoggingLevel) bool` - Return true if logs will be generated at the given level


## Future APIs

The APIs available today are just a start. Future APIs will allow chaincode to query transactions, blocks, and possibly previous state. Open an issue in the [repository](https://github.com/hyperledger/fabric/issues) to add your support for APIs you would like to see.
