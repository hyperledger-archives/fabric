# Running the sdk tests

## Installing typescript and the sdk dependencies

We assume that nodejs (>=v6.1.0) and the relative packages are already installed.

### Typescript (>=1.8.10) and Typings (>=1.0.4)
```
sudo npm install -g typescript
sudo npm install -g typings

```

### Dependencies

Fetch dependent modules (package.json) and type information (typings.json):

```
npm install
typings install

```

### Compile

Compile the Typescript code (from the *fabric/sdk/node* folder, reads tsconfig.json):
```
tsc
```

## Running the unit tests

### Setting the environment

We assume the sandbox setting is used as described here: [SanboxSetup.md](https://github.com/hyperledger/fabric/blob/master/docs/API/SandboxSetup.md#vagrant-terminal-2-chaincode).

We also assume that the fabric is running at security level 256 (default).

Don't forget to enable security and privacy as described in [SanboxSetup.md](https://github.com/hyperledger/fabric/blob/master/docs/API/SandboxSetup.md#vagrant-terminal-2-chaincode). 

### chain-tests

This test case exercises chaincode *chaincode_example02* as described in in [SanboxSetup.md](https://github.com/hyperledger/fabric/blob/master/docs/API/SandboxSetup.md#vagrant-terminal-2-chaincode). 

To run the 'chain-tests', run the following command (from the *fabric/sdk/node* folder):


```
rm -rf /tmp/keyValStore/ && node test/unit/chain-tests.js
```

### asset-mgmt

This test case exercises chaincode *asset_management*. When runnign the chiancode as described in [SanboxSetup.md](https://github.com/hyperledger/fabric/blob/master/docs/API/SandboxSetup.md#vagrant-terminal-2-chaincode), please name it *assetmgmt*, this is the name used in unit tests.

To run the *asset-mgmt*, To run the *asset-mgmt*, from the *node* folder run the following command:

```
rm -rf /tmp/keyValStore/ && node test/unit/asset=mgmt.js
```
