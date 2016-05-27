/**
 * Simple asset management use case where authentication is performed
 * with the help of TCerts only (use-case 1) or attributes only (use-case 2).*/

var hlc = require('../..');
var test = require('tape');
var util = require('util');
var fs = require('fs');

// constants
var registrar = {
    name: 'WebAppAdmin',
    secret: 'DJY27pEnl16d'
};

var alice, bob, chaincodeId;
var deltaAB = "1";

//
//  Create and configure a test chain
//
var chain = hlc.newChain("testChain");
chain.setKeyValStore(hlc.newFileKeyValStore('/tmp/keyValStore'));
chain.setMemberServicesUrl("grpc://localhost:50051");
chain.addPeer("grpc://localhost:30303");
chain.setDevMode(true);

test('Enroll the registrar', function (t) {
    // Get the WebAppAdmin member
    chain.getUser(registrar.name, function (err, user) {
        if (err) return fail(t, "get registrar", err);
        // Enroll the WebAppAdmin user with the certificate authority using
        // the one time password hard coded inside the membersrvc.yaml.
        user.enroll(registrar.secret, function (err) {
            if (err) return fail(t, "enroll registrar", err);
            registrar.user = user;
            pass(t, "enrolled " + registrar.name);
        });
    });
});

test('Enroll Alice', function (t) {
    getUser('Alice', function (err, user) {
        if (err) return fail(t, "enroll Alice", err);
        alice = user;
        pass(t, "enroll Alice");
    });
});

test('Enroll Bob', function (t) {
    getUser('Bob', function (err, user) {
        if (err) return fail(t, "enroll Bob", err);
        bob = user;
        pass(t, "enroll Bob");
    });
});

test("Alice deploys chaincode", function (t) {
    var deployRequest = {
        name: "mycc",
        function: 'init',
        arguments: ['a', '100', 'b', '200'],
        confidential: true
    };
    var tx = alice.deploy(deployRequest);
    tx.on('submitted', function (results) {
        // chaincodeId = results;
        chaincodeId = "mycc";   // TODO: dev mode

        // TODO: pass should be in the complete once it is done
        pass(t, "Alice deploy chaincode. ID " + chaincodeId);
    });
    tx.on('complete', function (results) {
        console.log("deploy complete: %j", results);
    });
    tx.on('error', function (err) {
        fail(t, "Alice depoy chaincode", err);
    });
});


test("Bob invokes chaincode", function (t) {
    var invokeRequest = {
        // Name (hash) required for invoke
        name: chaincodeId,
        // Function to trigger
        function: "invoke",
        // Parameters for the invoke function
        arguments: ["a", "b", deltaAB],
        confidential: true
    };
    var tx = bob.invoke(invokeRequest);
    tx.on('submitted', function () {
        console.log("query submitted");

        // TODO: pass should be in the complete once it is done
        pass(t, "Bob invoke");
    });
    tx.on('complete', function (results) {
        console.log("invoke completed");
    });
    tx.on('error', function (err) {
        fail(t, "Bob invoke", err);
    });
});

test("Alice queries chaincode", function (t) {
    var queryRequest = {
        // Name (hash) required for query
        name: chaincodeId,
        // Function to trigger
        function: "query",
        // Existing state variable to retrieve
        arguments: ["a"],
        confidential: true
    };
    var tx = alice.query(queryRequest);
    tx.on('submitted', function () {
        console.log("query submitted");
    });
    tx.on('complete', function (results) {
        pass(t, "Alice query. Result : " + results);
    });
    tx.on('error', function (err) {
        fail(t, "Alice query", err);
    });
});

/**
 * Get the user and if not enrolled, register and enroll the user.
 */
function getUser(name, cb) {
    chain.getUser(name, function (err, user) {
        if (err) return cb(err);
        if (user.isEnrolled()) return cb();
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
    t.pass("Failure: [" + msg + "]: [" + err + "]");
    t.end(err);
}
