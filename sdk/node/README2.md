# Running the sdk tests

## Installing typescript and the sdk dependencies

We assume that nodejs and the relative packages are already installed.

### Typescript
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

Compile the Typescript code (must be in this dir, reads tsconfig.json):
```
tsc
```
## Running the unit tests

### Setting the environment

We assume the sandbox setting is used as described here: [SanboxSetup.md](https://github.com/hyperledger/fabric/blob/master/docs/API/SandboxSetup.md#vagrant-terminal-2-chaincode).

We also assume that the fabric is running at security level 384.

To enforce this, please, make ensure that in the *core.yaml* file the security level property is set as follows:
```
security:
    # Can be 256 or 384. If you change here, you have to change also
    # the same property in membersrvc.yaml to the same value
    level: 384
```
The same in file *membersrvc.yaml* as well.

Don't forget to enable security and privacy as described in [SanboxSetup.md](https://github.com/hyperledger/fabric/blob/master/docs/API/SandboxSetup.md#vagrant-terminal-2-chaincode). 

### UC1

This test case exercises chaincode *chaincode_example02* as described in in [SanboxSetup.md](https://github.com/hyperledger/fabric/blob/master/docs/API/SandboxSetup.md#vagrant-terminal-2-chaincode). 

To run the 'uc1', from the *node* folder run the following command:
```
node test/unit/uc1.js
```

### UC2

This test case exercises chaincode *asset_management*. When runnign the chiancode as described in [SanboxSetup.md](https://github.com/hyperledger/fabric/blob/master/docs/API/SandboxSetup.md#vagrant-terminal-2-chaincode), please name it *assetmgm*, this is the name used in uc2.

To run the 'uc2', from the *node* folder run the following command:
```
node test/unit/uc2.js
```
