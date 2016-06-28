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

/**
 * Simple asset management use case where authentication is performed
 * with the help of TCerts only (use-case 1) or attributes only (use-case 2).*/

var hfc = require('../..');
var test = require('tape');
var util = require('util');
var fs = require('fs');

// constants
var registrar = {
    name: 'WebAppAdmin',
    secret: 'DJY27pEnl16d'
};

var alice, bob, charlie;
var alicesCert, bobAppCert, charlieAppCert;

// Path to the local directory containing the chaincode project under $GOPATH
var testChaincodePath = "github.com/asset_management/";

// Chaincode hash that will be filled in by the deployment operation or
// chaincode name that will be referenced in development mode.
var testChaincodeName = "mycc2";

// testChaincodeID will store the chaincode ID value after deployment.
var testChaincodeID;

//
//  Create and configure a test chain
//
var chain = hfc.newChain("testChain");
chain.setKeyValStore(hfc.newFileKeyValStore('/tmp/keyValStore'));
chain.setMemberServicesUrl("grpc://localhost:50051");
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


/**
 * Get the user and if not enrolled, register and enroll the user.
 */
function getUser(name, cb) {
    chain.getUser(name, function (err, user) {
        if (err) return cb(err);
        if (user.isEnrolled()) return cb(null, user);
        // User is not enrolled yet, so perform both registration and enrollment
        var registrationRequest = {
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

test('Enroll the registrar', function (t) {
    // Get the WebAppAdmin member
    chain.getUser(registrar.name, function (err, user) {
        if (err) return fail(t, "get registrar", err);
        // Enroll the WebAppAdmin user with the certificate authority using
        // the one time password hard coded inside the membersrvc.yaml.
        user.enroll(registrar.secret, function (err) {
            if (err) return fail(t, "enroll registrar", err);
            chain.setRegistrar(user);
            pass(t, "enrolled " + registrar.name);
        });
    });
});

test('Enroll Alice', function (t) {
    getUser('Alice', function (err, user) {
        if (err) return fail(t, "enroll Alice", err);
        alice = user;
        alice.getUserCert(null, function (err, userCert) {
            if (err) fail(t, "Failed getting Application certificate.");
            alicesCert = userCert;
            pass(t, "enroll Alice");
        })
    });
});

test('Enroll Bob', function (t) {
    getUser('Bob', function (err, user) {
        if (err) return fail(t, "enroll Bob", err);
        bob = user;
        bob.getUserCert(null, function (err, userCert) {
            if (err) fail(t, "Failed getting Application certificate.");
            bobAppCert = userCert;
            pass(t, "enroll Bob");
        })
    });
});

test('Enroll Charlie', function (t) {
    getUser('Charlie', function (err, user) {
        if (err) return fail(t, "enroll Charlie", err);
        charlie = user;
        charlie.getUserCert(null, function (err, userCert) {
            if (err) fail(t, "Failed getting Application certificate.");
            charlieAppCert = userCert;
            pass(t, "enroll Charlie");
        })
    });
});

test("Alice deploys chaincode", function (t) {
    t.plan(1);

    console.log('Deploy and assigning administrative rights to Alice [%s]', alicesCert.encode().toString('hex'));

    // Construct the deploy request
    var deployRequest = {
      // Function to trigger
      fcn: "init",
      // Arguments to the initializing function
      args: [],
      // Mark chaincode as confidential
      confidential: true,
      // Assign Alice's cert
      metadata: alicesCert.encode()
    };

    if (mode === 'dev') {
        // Name required for deploy in development mode
        deployRequest.chaincodeName = testChaincodeName;
    } else {
        // Path (under $GOPATH) required for deploy in network mode
        deployRequest.chaincodePath = testChaincodePath;
    }

    // Trigger the deploy transaction
    var deployTx = alice.deploy(deployRequest);

    // Print the deploy results
    deployTx.on('complete', function(results) {
      // Deploy request completed successfully
      console.log(util.format("deploy results: %j", results));
      // Set the testChaincodeID for subsequent tests
      testChaincodeID = results.chaincodeID;
      console.log("testChaincodeID:" + testChaincodeID);
      t.pass(util.format("Successfully deployed chaincode: request=%j, response=%j", deployRequest, results));
    });
    deployTx.on('error', function(err) {
      // Deploy request failed
      t.fail(util.format("Failed to deploy chaincode: request=%j, error=%j", deployRequest, err));
    });
});


test("Alice assign ownership", function (t) {
    t.plan(1);

    console.log("Chaincode ID: %s", testChaincodeID);

    var invokeRequest = {
        // Name (hash) required for invoke
        chaincodeID: testChaincodeID,
        // Function to trigger
        fcn: "assign",
        // Parameters for the invoke function
        args: ['Ferrari', bobAppCert.encode().toString('base64')],
        confidential: true,
        userCert: alicesCert
    };

    var tx = alice.invoke(invokeRequest);
    tx.on('submitted', function (results) {
        // Invoke transaction submitted successfully
        console.log("Successfully submitted chaincode invoke transaction");
    });
    tx.on('complete', function (results) {
        console.log("invoke completed");
        pass(t, "Alice invoke");
    });
    tx.on('error', function (err) {
        fail(t, "Alice invoke", err);
    });
});

test("Bob transfers ownership to Charlie", function (t) {
    t.plan(1);

    var invokeRequest = {
        // Name (hash) required for invoke
        chaincodeID: testChaincodeID,
        // Function to trigger
        fcn: "transfer",
        // Parameters for the invoke function
        args: ['Ferrari', charlieAppCert.encode().toString('base64')],
        userCert: bobAppCert,
        confidential: true
    };

    var tx = bob.invoke(invokeRequest);
    tx.on('submitted', function (results) {
        // Invoke transaction submitted successfully
        console.log("Successfully submitted chaincode invoke transaction");
    });
    tx.on('complete', function (results) {
        console.log("invoke completed");
        pass(t, "Bob invoke");
    });
    tx.on('error', function (err) {
        fail(t, "Bob invoke", err);
    });
});

test("Alice queries chaincode", function (t) {
    t.plan(1);

    console.log("Alice queries chaincode: " + testChaincodeID);

    var queryRequest = {
        // Name (hash) required for query
        chaincodeID: testChaincodeID,
        // Function to trigger
        fcn: "query",
        // Existing state variable to retrieve
        args: ["Ferrari"],
        confidential: true
    };

    var tx = alice.query(queryRequest);
    tx.on('complete', function (results) {
        console.log(util.format('Results: %j', results));
        console.log(util.format('Charlie identity: %s', charlieAppCert.encode().toString('hex')));

        if (results.result != charlieAppCert.encode().toString('hex')) {
            fail(t, "Charlie is not the owner of the asset.")
        }
        pass(t, "Alice query. Result: " + results);
    });
    tx.on('error', function (err) {
        fail(t, "Alice query", err);
    });
});
