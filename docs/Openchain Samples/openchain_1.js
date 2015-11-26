// Initialize the Swagger JS plugin
var client = require('swagger-client');

// Point plugin to the location of the Swagger API descrition
var api_url = 'http://localhost:5554/rest_api.json';

// Initialize the Swagger-based client, passing in the API URL
var swagger = new client({
  url: api_url,
  success: function() { // If connection to API established, sucess and proceed
    console.log("Connected to REST API server!\n");

    // GET /chain
    swagger.Blockchain.getChain({},{responseContentType: 'application/json'}, function(Blockchain){
        console.log("----- Blockchain Retrieved: -----\n");
        console.log(Blockchain);
        console.log("----------\n\n");
    });

    // GET /chain/blocks/{Block} -- success
    swagger.Block.getBlock({'Block': '0'},{responseContentType: 'application/json'}, function(Block){
        console.log("----- Block Retrieved: -----\n");
        console.log(Block);
        console.log("----------\n\n");
    });

    // GET /chain/blocks/{Block} -- failure, block not in test blockchain
    swagger.Block.getBlock({'Block': '5'},{responseContentType: 'application/json'}, function(Block){
        console.log("----- Block Retrieved: -----\n");
        console.log(Block);
        console.log("----------\n\n");
    });

    // /state/{chaincodeID}/{key} -- success/match, chaincode with this key exists
    swagger.State.getChaincodeState({'chaincodeID': 'MyContract', 'key': 'x'},{responseContentType: 'application/json'}, function(State){
        console.log("----- State Retrieved: -----\n");
        console.log(State);
        console.log("----------\n\n");
    });

    // /state/{chaincodeID}/{key} -- success/no match, chaincode with this key not found
    swagger.State.getChaincodeState({'chaincodeID': 'MyOtherContract', 'key': 'NONE'},{responseContentType: 'application/json'}, function(State){
        console.log("----- State Retrieved: -----\n");
        console.log(State);
        console.log("----------\n\n");
    });

    // POST /devops/build -- success
    var chaincodeSpec = {
        type: "GOLANG",
        chaincodeID: {
            url: "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example01",
            version: "0.1.0"
        }
    };

    // Print confirmation message only, as codePackage is too large to be printed
    swagger.Devops.chaincodeBuild({'ChaincodeSpec': chaincodeSpec},{responseContentType: 'application/json'},function(Devops){
        console.log("----- Devops Build Triggered: -----\n");
        console.log(Devops);
        console.log("----------\n\n");
    });

    // POST /devops/build -- failure, only GOLANG support for chainCode
    var chaincodeSpec = {
        type: "NODE",
        chaincodeID: {
            url: "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_simple",
            version: "0.1.0"
        }
    };

    // Print confirmation message only, as codePackage is too large to be printed
    swagger.Devops.chaincodeBuild({'ChaincodeSpec': chaincodeSpec},{responseContentType: 'application/json'},function(Devops){
        console.log("----- Devops Build Triggered: -----\n");
        console.log(Devops);
        console.log("----------\n\n");
    });

    // POST /devops/build -- failure, chainCode path does not exist
    var chaincodeSpec = {
        type: "GOLANG",
        chaincodeID: {
            url: "/test/test/test",
            version: "0.1.0"
        }
    };

    // Print confirmation message only, as codePackage is too large to be printed
    swagger.Devops.chaincodeBuild({'ChaincodeSpec': chaincodeSpec},{responseContentType: 'application/json'},function(Devops){
        console.log("----- Devops Build Triggered: -----\n");
        console.log(Devops);
        console.log("----------\n\n");
    });
  },
  error: function() { // If connection to API failed, error and exit
    console.log("Failed to connect to REST API server.\n");
    console.log("Exiting.\n");
  }
});
