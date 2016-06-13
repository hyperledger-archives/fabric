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

var chain, admin, alice, bob, chaincodeID;

// Path to the local directory containing the chaincode project under $GOPATH
var testChaincodePath = "github.com/asset_management_with_roles";

// Chaincode name/hash that will be filled in by the deployment operation
var testChaincodeName = "";

// Create the chain and set users admin, alice, and bob
function runTest(cb) {
   console.log("initializing ...");
   var chain = hlc.newChain("testChain");
   chain.setKeyValStore(hlc.newFileKeyValStore("/tmp/keyValStore"));
   chain.setMemberServicesUrl("grpc://localhost:50051");
   chain.addPeer("grpc://localhost:30303");
   chain.setDevMode(false);
   console.log("enrolling WebAppAdmin ...");
   chain.enroll("WebAppAdmin", "DJY27pEnl16d", function (err, user) {
      if (err) return cb(err);
      admin = user;
      chain.setRegistrar(admin);
      // Register and enroll webAdmin
      console.log("registering Alice ...");
      var rr = {
          enrollmentID: "Alice",
          account: "bank_a",
          affiliation: "00001"
      }
      chain.registerAndEnroll(rr, function(err,user) {
         if (err) return cb(err);
         alice = user;
         console.log("registering Bob ...");
         rr.enrollmentID = "Bob",
         chain.registerAndEnroll(rr, function(err,user) {
            if (err) return cb(err);
            bob = user;
            console.log("initialized");
            return deploy(cb);
         });
      });
   });
}

// Deploy assetmgmt_with_roles with the name of the assigner role in the metadata
function deploy(cb) {
    console.log("deploying with the role name 'assigner' in metadata ...");
    var req = {
        chaincodePath: testChaincodePath,
        fcn: "init",
        args: [],
        confidential: true,
        metadata: "assigner"
    };

    var tx = admin.deploy(req);
    tx.on('submitted', function (results) {
        console.log("deploy chaincode submitted");
    });
    tx.on('complete', function (results) {
        // Deploy request completed successfully
        console.log("deploy results: %j", results);
        // Set the chaincode name (hash returned) for subsequent tests
        testChaincodeName = results.chaincodeID;
        console.log("testChaincodeName:" + testChaincodeName);

        return assign(cb);
    });
    tx.on('error', function (err) {
        console.log("deploy error: %j", err.toString());
        return cb(err);
    });
}

function assign(cb) {
    var req = {
        chaincodeID: testChaincodeName,
        fcn: "assign",
        args: ["MyAsset","ownerID"],
        confidential: true,
        attrs: ['role']
    };
    var tx = admin.invoke(req);
    tx.on('submitted', function (results) {
        console.log("assign transaction ID: %j", results);
    });
    tx.on('complete', function (results) {
        console.log("assign invoke complete: %j", results);
        return cb();
    });
    tx.on('error', function (err) {
        console.log("assign invoke error: %j", err);
        return cb(err);
    });
}

test('asset management with roles', function (t) {
    runTest(function(err) {
        if (err) {
            t.fail("error: "+err.toString());
            t.end(err);
        } else {
            t.pass("successfully ran asset management with roles test");
            t.end();
        }
    });
});
