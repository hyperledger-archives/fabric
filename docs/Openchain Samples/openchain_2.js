// Read in the Openchain API functions from the compiled TypeScript file
var openchain = require("./built/api.js");

/**********

  Block Related APIs
  ------------------

**********/

// Initialize BlockApi class, which contains the Block related
// methods.

// Initialize the constructor with an empty url string, as default address
// of the local peer node is already set inside the api.js to
// 'http://127.0.0.1:3000'.
var blockchain = new openchain.BlockApi('');

// The api.js exposes the APIs with the use of Promises as async callbacks.

// Query the getBlock() API with Promises to retrive the contents of
// Block 3 in the blockchain.
var promise = blockchain.getBlock(3);

// Once promise is fulfilled, print the contents of the response body.
promise.then(function(block) {
    console.log('Current Block Contents:\n');
    console.log('-----------------------\n');

    console.log(block.body);
    console.log('\n');
    return;
});

/**********

  Blockchain Related APIs
  -----------------------

**********/

// Initialize BlockchainApi class, which contains the blockchain related
// methods.

// Initialize the constructor with an empty url string, as default address
// of the local peer node is already set inside the api.js to
// 'http://127.0.0.1:3000'.
var blockchain = new openchain.BlockchainApi('');

// The api.js exposes the APIs with the use of Promises as async callbacks.

// Query the getChain() API with Promises.
var promise = blockchain.getChain();

// Once promise is fulfilled, print the contents of the response body.
promise.then(function(blockchain) {
    console.log('Current Blockchain Contents:\n');
    console.log('----------------------------\n');

    console.log(blockchain.body);
    console.log('\n');
    return;
});

/**********

  State Related APIs
  ------------------

**********/

// Initialize StateApi class, which contains the State related
// methods.

// Initialize the constructor with an empty url string, as default address
// of the local peer node is already set inside the api.js to
// 'http://127.0.0.1:3000'.
var blockchain = new openchain.StateApi('');

// The api.js exposes the APIs with the use of Promises as async callbacks.

// Query the getChaincodeState() API with Promises.
var promise = blockchain.getChaincodeState('MyContract', 'x');

// Once promise is fulfilled, print the contents of the response body.
promise.then(function(state) {
    console.log('State Of Chaincode:\n');
    console.log('-------------------\n');

    console.log(state.body);
    console.log('\n');
    return;
});

/**********

  Devops Related APIs
  -------------------

**********/

// Initialize DevopsApi class, which contains the Devops related
// methods.

// Initialize the constructor with an empty url string, as default address
// of the local peer node is already set inside the api.js to
// 'http://127.0.0.1:3000'.
var blockchain = new openchain.DevopsApi('');

// The api.js exposes the APIs with the use of Promises as async callbacks.

// Query the chaincodeBuild() API with Promises.
var chaincodeSpec = {
    type: "GOLANG",
    chaincodeID: {
        url: "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example01",
        version: "0.1.0"
    }
};
var promise = blockchain.chaincodeBuild(chaincodeSpec);

// Once promise is fulfilled, print the contents of the response body.
promise.then(function(devops) {
    console.log('Result of Devops Build:\n');
    console.log('-----------------------\n');

    // Print confirmation message only, as codePackage is too large to be printed
    console.log(devops.body);
    console.log('\n');
    return;
});
