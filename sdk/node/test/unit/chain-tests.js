/**
 * Copyright 2016 IBM
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */
/**
 * Licensed Materials - Property of IBM
 * © Copyright IBM Corp. 2016
 */

var hfc = require('../..');
var test = require('tape');
var util = require('util');
var fs = require('fs');

//
//  Create a test chain
//

var chain = hfc.newChain("testChain");

var registrar = {
    name: 'WebAppAdmin',
    secret: 'DJY27pEnl16d'
};


//
// Configure the test chain
//
// Set the directory for the local file-based key value store, point to the
// address of the membership service, and add an associated peer node.
//
// If the "tlsca.cert" file exists then the client-sdk will
// try to connect to the member services using TLS.
// The "tlsca.cert" is supposed to contain the root certificate (in PEM format)
// to be used to authenticate the member services certificate.
//

chain.setKeyValStore(hfc.newFileKeyValStore('/tmp/keyValStore'));
if (fs.existsSync("tlsca.cert")) {
    chain.setMemberServicesUrl("grpcs://localhost:50051", fs.readFileSync('tlsca.cert'));
} else {
    chain.setMemberServicesUrl("grpc://localhost:50051");
}
chain.addPeer("grpc://localhost:30303");

//
// Set the chaincode deployment mode to either developent mode (user runs chaincode)
// or network mode (code package built and sent to the peer).
//

var mode =  process.env['DEPLOY_MODE'];
console.log("$DEPLOY_MODE: " + mode);
if (mode === 'dev') {
    chain.setDevMode(true);
} else {
    chain.setDevMode(false);
}

//
// Configure test users
//
// Set the values required to register a user with the certificate authority.
//

test_user1 = {
    name: "WebApp_user1",
    role: 1, // Client
    account: "bank_a",
    affiliation: "00001"
};

//
// Declare variables to store the test user Member objects returned after
// registration and enrollment as they will be used across multiple tests.
//

var test_user_Member1;

//
// Declare test variables that will be used to store chaincode values used
// across multiple tests.
//

// Path to the local directory containing the chaincode project under $GOPATH
var testChaincodePath = "github.com/chaincode_example02/";

// Chaincode hash that will be filled in by the deployment operation or
// chaincode name that will be referenced in development mode.
var testChaincodeName = "mycc1";

// testChaincodeID will store the chaincode ID value after deployment.
var testChaincodeID;

// Initializing values for chaincode parameters
var initA = "100";
var initB = "200";
var deltaAB = "1";

function getUser(name, cb) {
    chain.getUser(name, function (err, user) {
        if (err) return cb(err);
        if (user.isEnrolled()) return cb(null,user);
        // User is not enrolled yet, so perform both registration and enrollment
        var registrationRequest = {
            registrar: registrar.user,
            enrollmentID: name,
            account: "bank_a",
            affiliation: "00001"
        };
        user.registerAndEnroll(registrationRequest, function (err) {
            if (err) cb(err, null)
            cb(null, user)
        });
    });
}

function pass(t, msg) {
    t.pass("Success: [" + msg + "]");
    t.end();
}

function fail(t, msg, err) {
    t.fail("Failure: [" + msg + "]: [" + err + "]");
    t.end(err);
}

//
// Enroll the WebAppAdmin member. WebAppAdmin member is already registered
// manually by being included inside the membersrvc.yaml file.
//

test('Enroll WebAppAdmin', function (t) {
    t.plan(3);

    // Get the WebAppAdmin member
    chain.getMember("WebAppAdmin", function (err, WebAppAdmin) {
        if (err) {
            t.fail("Failed to get WebAppAdmin member " + " ---> " + err);
            t.end(err);
        } else {
            t.pass("Successfully got WebAppAdmin member" /*+ " ---> " + JSON.stringify(crypto)*/);

            // Enroll the WebAppAdmin member with the certificate authority using
            // the one time password hard coded inside the membersrvc.yaml.
            pw = "DJY27pEnl16d";
            WebAppAdmin.enroll(pw, function (err, crypto) {
                if (err) {
                    t.fail("Failed to enroll WebAppAdmin member " + " ---> " + err);
                    t.end(err);
                } else {
                    t.pass("Successfully enrolled WebAppAdmin member" /*+ " ---> " + JSON.stringify(crypto)*/);

                    // Confirm that the WebAppAdmin token has been created in the key value store
                    path = chain.getKeyValStore().dir + "/member." + WebAppAdmin.getName();

                    fs.exists(path, function (exists) {
                        if (exists) {
                            t.pass("Successfully stored client token" /*+ " ---> " + WebAppAdmin.getName()*/);
                        } else {
                            t.fail("Failed to store client token for " + WebAppAdmin.getName() + " ---> " + err);
                        }
                    });
                }
            });
        }
    });
});

//
// Set the WebAppAdmin as the designated chain 'registrar' member who will
// subsequently register/enroll other new members. WebAppAdmin member is already
// registered manually by being included inside the membersrvc.yaml file and
// enrolled in the UT above.
//

test('Set chain registrar', function (t) {
    t.plan(2);

    // Get the WebAppAdmin member
    chain.getMember("WebAppAdmin", function (err, WebAppAdmin) {
        if (err) {
            t.fail("Failed to get WebAppAdmin member " + " ---> " + err);
            t.end(err);
        } else {
            t.pass("Successfully got WebAppAdmin member");

            // Set the WebAppAdmin as the designated chain registrar
            chain.setRegistrar(WebAppAdmin);

            // Confirm that the chain registrar is now WebAppAdmin
            t.equal(chain.getRegistrar().getName(), "WebAppAdmin", "Successfully set chain registrar");
        }
    });
});

//
// Register and enroll a new user with the certificate authority.
// This will be performed by the registrar member, WebAppAdmin.
//

test('Register and enroll a new user', function (t) {
    t.plan(2);

    // Register and enroll test_user
    getUser(test_user1.name, function (err, user) {
        if (err) {
            fail(t, "Failed to get " + test_user1.name + " ---> ", err);
        } else {
            test_user_Member1 = user;

            t.pass("Successfully registered and enrolled " + test_user_Member1.getName());

            // Confirm that the user token has been created in the key value store
            path = chain.getKeyValStore().dir + "/member." + test_user1.name;
            fs.exists(path, function (exists) {
                if (exists) {
                    t.pass("Successfully stored client token" /*+ " ---> " + test_user1.name*/);
                    t.end()
                } else {
                    t.fail("Failed to store client token for " + test_user1.name + " ---> " + err);
                    t.end(err)
                }
            });
        }
    });
});

//
// Create and issue a chaincode deploy request by the test user, who was
// registered and enrolled in the UT above. Deploy a testing chaincode from
// a local directory in the user's $GOPATH.
//

test('Deploy a chaincode by enrolled user', function(t) {
  t.plan(1);

  // Construct the deploy request
  var deployRequest = {
    // Function to trigger
    fcn: "init",
    // Arguments to the initializing function
    args: ["a", initA, "b", initB]
  };

  if (mode === 'dev') {
      // Name required for deploy in development mode
      deployRequest.chaincodeName = testChaincodeName;
  } else {
      // Path (under $GOPATH) required for deploy in network mode
      deployRequest.chaincodePath = testChaincodePath;
  }

  // Trigger the deploy transaction
  var deployTx = test_user_Member1.deploy(deployRequest);

  // Print the deploy results
  deployTx.on('complete', function(results) {
    // Deploy request completed successfully
    console.log(util.format("deploy results: %j",results));
    // Set the testChaincodeID for subsequent tests
    testChaincodeID = results.chaincodeID;
    console.log("testChaincodeID:" + testChaincodeID);
    t.pass(util.format("Successfully deployed chaincode: request=%j, response=%j", deployRequest, results));
  });
  deployTx.on('error', function(err) {
    // Deploy request failed
    t.fail(util.format("Failed to deploy chaincode: request=%j, error=%j",deployRequest,err));
  });
});

//
// Create and issue a chaincode query request by the test user, who was
// registered and enrolled in the UT above. Query an existing chaincode
// state variable with a transaction certificate batch size of 1.
//

test('Query existing chaincode state by enrolled user with batch size of 1', function (t) {
    t.plan(1);

    // Construct the query request
    var queryRequest = {
        // Name (hash) required for query
        chaincodeID: testChaincodeID,
        // Function to trigger
        fcn: "query",
        // Existing state variable to retrieve
        args: ["a"]
    };

    // Trigger the query transaction
    test_user_Member1.setTCertBatchSize(1);
    var queryTx = test_user_Member1.query(queryRequest);

    // Print the query results
    queryTx.on('complete', function (results) {
        // Query completed successfully
        t.pass(util.format("Successfully queried existing chaincode state: request=%j, response=%j, value=%s", queryRequest, results, results.result.toString()));
    });
    queryTx.on('error', function (err) {
        // Query failed
        t.fail(util.format("Failed to query existing chaincode state: request=%j, error=%j", queryRequest, err));
    });
});

//
// Create and issue a chaincode query request by the test user, who was
// registered and enrolled in the UT above. Query an existing chaincode
// state variable with a transaction certificate batch size of 100.
//

test('Query existing chaincode state by enrolled user with batch size of 100', function (t) {
    t.plan(1);

    // Construct the query request
    var queryRequest = {
        // Name (hash) required for query
        chaincodeID: testChaincodeID,
        // Function to trigger
        fcn: "query",
        // Existing state variable to retrieve
        args: ["a"]
    };

    // Trigger the query transaction
    test_user_Member1.setTCertBatchSize(100);
    var queryTx = test_user_Member1.query(queryRequest);

    // Print the query results
    queryTx.on('complete', function (results) {
        // Query completed successfully
        t.pass(util.format("Successfully queried existing chaincode state: request=%j, response=%j, value=%s", queryRequest, results, results.result.toString()));
    });
    queryTx.on('error', function (err) {
      // Query failed
      t.fail(util.format("Failed to query existing chaincode state: request=%j, error=%j", queryRequest, err));
    });
});

//
// Create and issue a chaincode query request by the test user, who was
// registered and enrolled in the UT above. Query a non-existing chaincode
// state variable.
//

test('Query non-existing chaincode state by enrolled user', function (t) {
    t.plan(1);

    // Construct the query request
    var queryRequest = {
        // Name (hash) required for query
        chaincodeID: testChaincodeID,
        // Function to trigger
        fcn: "query",
        // Existing state variable to retrieve
        args: ["BOGUS"]
    };

    // Trigger the query transaction
    var queryTx = test_user_Member1.query(queryRequest);

    // Print the query results
    queryTx.on('complete', function (results) {
        // Query completed successfully
        t.fail(util.format("Successfully queried non-existing chaincode state: request=%j, response=%j, value=%s", queryRequest, results, results.result.toString()));
    });
    queryTx.on('error', function (err) {
        // Query failed
        t.pass(util.format("Failed to query non-existing chaincode state: request=%j, error=%j",queryRequest,err));
    });
});

//
// Create and issue a chaincode query request by the test user, who was
// registered and enrolled in the UT above. Query a non-existing chaincode
// function.
//

test('Query non-existing chaincode function by enrolled user', function (t) {
    t.plan(1);

    // Construct the query request
    var queryRequest = {
        // Name (hash) required for query
        chaincodeID: testChaincodeID,
        // Function to trigger
        fcn: "BOGUS",
        // Existing state variable to retrieve
        args: ["a"]
    };

    // Trigger the query transaction
    var queryTx = test_user_Member1.query(queryRequest);

    // Print the query results
    queryTx.on('complete', function (results) {
        // Query completed successfully
        t.fail(util.format("Successfully queried non-existing chaincode function: request=%j, response=%j, value=%s", queryRequest, results, results.result.toString()));
    });
    queryTx.on('error', function (err) {
        // Query failed
        t.pass(util.format("Failed to query non-existing chaincode function: request=%j, error=%j",queryRequest,err));
    });
});

//
// Create and issue a chaincode invoke request by the test user, who was
// registered and enrolled in the UT above.
//

test('Invoke a chaincode by enrolled user', function (t) {
    t.plan(1);

    // Construct the invoke request
    var invokeRequest = {
        // Name (hash) required for invoke
        chaincodeID: testChaincodeID,
        // Function to trigger
        fcn: "invoke",
        // Parameters for the invoke function
        args: ["a", "b", deltaAB]
    };

    // Trigger the invoke transaction
    var invokeTx = test_user_Member1.invoke(invokeRequest);

    // Print the invoke results
    invokeTx.on('submitted', function (results) {
        // Invoke transaction submitted successfully
        t.pass(util.format("Successfully submitted chaincode invoke transaction: request=%j, response=%j", invokeRequest,results));
    });
    invokeTx.on('error', function (err) {
        // Invoke transaction submission failed
        t.fail(util.format("Failed to submit chaincode invoke transaction: request=%j, error=%j", invokeRequest, err));
    });
});
