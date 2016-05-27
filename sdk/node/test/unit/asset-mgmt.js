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

var alice, bob, charlie, chaincodeId;
var alicesCert, bobAppCert, charlieAppCert;

//
//  Create and configure a test chain
//
var chain = hlc.newChain("testChain");
chain.setKeyValStore(hlc.newFileKeyValStore('/tmp/keyValStore'));
chain.setMemberServicesUrl("grpc://localhost:50051");
chain.addPeer("grpc://localhost:30303");

/**
 * Get the user and if not enrolled, register and enroll the user.
 */
function getUser(name, cb) {
    chain.getUser(name, function (err, user) {
        if (err) return cb(err);
        if (user.isEnrolled()) return cb();
        // User is not enrolled yet, so perform both registration and enrollment
        var registrationRequest = {
            enrollmentID: name,
            account: "bank_a",
            affiliation: "00001",
            registrar: registrar.user,
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
            registrar.user = user;
            pass(t, "enrolled " + registrar.name);
        });
    });
});

test('Enroll Alice', function (t) {
    getUser('Alice', function (err, user) {
        if (err) return fail(t, "enroll Alice", err);
        alice = user;

        alice.getApplicationCertificate(function (err, appCert) {
            if (err) fail(t, "Failed getting Application certificate.");
            alicesCert = appCert;

            pass(t, "enroll Alice");
        })
    });
});

test('Enroll Bob', function (t) {
    getUser('Bob', function (err, user) {
        if (err) return fail(t, "enroll Bob", err);
        bob = user;

        bob.getApplicationCertificate(function (err, appCert) {
            if (err) fail(t, "Failed getting Application certificate.");
            bobAppCert = appCert;

            pass(t, "enroll Bob");
        })
    });
});

test('Enroll Charlie', function (t) {
    getUser('Charlie', function (err, user) {
        if (err) return fail(t, "enroll Charlie", err);
        charlie = user;

        charlie.getApplicationCertificate(function (err, appCert) {
            if (err) fail(t, "Failed getting Application certificate.");
            charlieAppCert = appCert;

            pass(t, "enroll Charlie");
        })
    });
});

test("Alice deploys chaincode", function (t) {
    console.log('Deploy and assigning administrative rights to Alice [%s]', alicesCert.encode().toString());
    var deployRequest = {
        chaincodeID: "assetmgmt",
        fcn: 'init',
        args: [],
        confidential: true,
        metadata: alicesCert.encode()
    };
    var tx = alice.deploy(deployRequest);
    tx.on('submitted', function (results) {
        // chaincodeId = results;
        chaincodeId = "assetmgmt";   // TODO: dev mode
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

test("Alice assign ownership", function (t) {
    var invokeRequest = {
        // Name (hash) required for invoke
        chaincodeId: chaincodeId,
        // Function to trigger
        fcn: "assign",
        // Parameters for the invoke function
        args: ['Ferrari', bobAppCert.encode().toString('base64')],
        confidential: true,
        userCert: alicesCert
    };

    var tx = alice.invoke(invokeRequest);
    tx.on('submitted', function () {
        console.log("query submitted");
        // TODO: pass should be in the complete once it is done
        pass(t, "Alice invoke");
    });
    tx.on('complete', function (results) {
        console.log("invoke completed");
    });
    tx.on('error', function (err) {
        fail(t, "Alice invoke", err);
    });
});

test("Bob transfers ownership to Charlie", function (t) {
    var invokeRequest = {
        // Name (hash) required for invoke
        name: chaincodeId,
        // Function to trigger
        function: "transfer",
        // Parameters for the invoke function
        arguments: ['Ferrari', charlieAppCert.encode().toString('base64')],

        invoker: {appCert: bobAppCert},
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
    sleep.sleep(5);
    var queryRequest = {
        // Name (hash) required for query
        name: chaincodeId,
        // Function to trigger
        function: "query",
        // Existing state variable to retrieve
        arguments: ["Ferrari"],
        confidential: true
    };
    var tx = alice.query(queryRequest);
    tx.on('submitted', function () {
        console.log("query submitted");
    });
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
