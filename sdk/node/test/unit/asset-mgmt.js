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

var hlc = require('../..');
var test = require('tape');
var util = require('util');
var fs = require('fs');
var sleep = require('sleep');

// constants
var registrar = {
    name: 'WebAppAdmin',
    secret: 'DJY27pEnl16d'
};

var alice, bob, charlie, chaincodeID;
var alicesCert, bobAppCert, charlieAppCert;

//
//  Create and configure a test chain
//
var chain = hlc.newChain("testChain");
chain.setKeyValStore(hlc.newFileKeyValStore('/tmp/keyValStore'));
chain.setMemberServicesUrl("grpc://localhost:50051");
chain.addPeer("grpc://localhost:30303");
chain.setDevMode(true);

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
    t.pass("Failure: [" + msg + "]: [" + err + "]");
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
        alice.getUserCert(function (err, userCert) {
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
        bob.getUserCert(function (err, userCert) {
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
        charlie.getUserCert(function (err, userCert) {
            if (err) fail(t, "Failed getting Application certificate.");
            charlieAppCert = userCert;
            pass(t, "enroll Charlie");
        })
    });
});

test("Alice deploys chaincode", function (t) {
    console.log('Deploy and assigning administrative rights to Alice [%s]', alicesCert.encode().toString('hex'));
    var deployRequest = {
        chaincodeID: "assetmgmt",
        fcn: 'init',
        args: [],
        confidential: true,
        metadata: alicesCert.encode()
    };
    var tx = alice.deploy(deployRequest);
    tx.on('submitted', function (results) {
        console.log("Chaincode ID: %s", results);
        chaincodeID = results.toString();
    });
    tx.on('complete', function (results) {
        console.log("deploy complete: %j", results);
        pass(t, "Alice deploy chaincode. ID: " + chaincodeID);
    });
    tx.on('error', function (err) {
        fail(t, "Alice depoy chaincode", err);
    });
});

test("Alice assign ownership", function (t) {
    console.log("Chaincode ID: %s", chaincodeID);
    var invokeRequest = {
        // Name (hash) required for invoke
        chaincodeID: chaincodeID,
        // Function to trigger
        fcn: "assign",
        // Parameters for the invoke function
        args: ['Ferrari', bobAppCert.encode().toString('base64')],
        confidential: true,
        userCert: alicesCert
    };

    var tx = alice.invoke(invokeRequest);
    tx.on('submitted', function () {
        console.log("invoke submitted");
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
    var invokeRequest = {
        // Name (hash) required for invoke
        chaincodeID: chaincodeID,
        // Function to trigger
        fcn: "transfer",
        // Parameters for the invoke function
        args: ['Ferrari', charlieAppCert.encode().toString('base64')],
        userCert: bobAppCert,
        confidential: true
    };

    var tx = bob.invoke(invokeRequest);
    tx.on('submitted', function () {
        console.log("query submitted");
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
    var queryRequest = {
        // Name (hash) required for query
        chaincodeID: chaincodeID,
        // Function to trigger
        fcn: "query",
        // Existing state variable to retrieve
        args: ["Ferrari"],
        confidential: true
    };
    var tx = alice.query(queryRequest);
    tx.on('complete', function (results) {
        console.log('Results          [%j]', results);
        console.log('Charlie identity [%s]', charlieAppCert.encode().toString('hex'));

        if (results != charlieAppCert.encode().toString('hex')) {
            fail(t, "Charlie is not the owner of the asset.")
        }
        pass(t, "Alice query. Result : " + results);
    });
    tx.on('error', function (err) {
        fail(t, "Alice query", err);
    });
});
