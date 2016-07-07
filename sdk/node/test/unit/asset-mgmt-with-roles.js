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
var crypto = require('../../lib/crypto');

var chain, chaincodeID;
var chaincodeName = "mycc3";
var deployer, alice, bob, assigner;
var aliceAccount = "12345-56789";
var bobAccount = "23456-67890";
var devMode = process.env.DEPLOY_MODE == 'dev';

// Create the chain and enroll users as deployer, assigner, and nonAssigner (who doesn't have privilege to assign.
function setup(cb) {
   console.log("initializing ...");
   var chain = hfc.newChain("testChain");
   chain.setKeyValStore(hfc.newFileKeyValStore("/tmp/keyValStore"));
   chain.setMemberServicesUrl("grpc://localhost:50051");
   chain.addPeer("grpc://localhost:30303");
   if (devMode) chain.setDevMode(true);
   console.log("enrolling deployer ...");
   chain.enroll("WebAppAdmin", "DJY27pEnl16d", function (err, user) {
      if (err) return cb(err);
      deployer = user;
      console.log("enrolling Assigner ...");
      chain.enroll("assigner","Tc43PeqBl11", function (err, user) {
         if (err) return cb(err);
         assigner = user;
         console.log("enrolling Alice ...");
         chain.enroll("alice","CMS10pEQlB16", function (err, user) {
            if (err) return cb(err);
            alice = user;
            console.log("enrolling Bob ...");
            chain.enroll("bob","NOE63pEQbL25", function (err, user) {
               if (err) return cb(err);
               bob = user;
               return deploy(cb);
            });
         });
      });
   });
}

// Deploy assetmgmt_with_roles with the name of the assigner role in the metadata
function deploy(cb) {
    console.log("deploying with the role name 'assigner' in metadata ...");
    var req = {
        fcn: "init",
        args: [],
        metadata: new Buffer("assigner")
    };
    if (devMode) {
       req.chaincodeName = chaincodeName;
    } else {
       req.chaincodePath = "github.com/asset_management_with_roles/";
    }
    var tx = deployer.deploy(req);
    tx.on('submitted', function (results) {
        console.log("deploy submitted: %j", results);
    });
    tx.on('complete', function (results) {
        console.log("deploy complete: %j", results);
        chaincodeID = results.chaincodeID;
        console.log("chaincodeID:" + chaincodeID);
        return cb();
    });
    tx.on('error', function (err) {
        console.log("deploy error: %j", err.toString());
        return cb(err);
    });
}

function assignOwner(user,owner,cb) {
    var req = {
        chaincodeID: chaincodeID,
        fcn: "assign",
        args: ["MyAsset",owner],
        attrs: ['role']
    };
    console.log("assign: invoking %j",req);
    var tx = user.invoke(req);
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

// Check to see if the owner of the asset is
function checkOwner(user,ownerAccount,cb) {
    var req = {
        chaincodeID: chaincodeID,
        fcn: "query",
        args: ["MyAsset"]
    };
    console.log("query: querying %j",req);
    var tx = user.query(req);
    tx.on('complete', function (results) {
       var realOwner = results.result;
       //console.log("realOwner: " + realOwner);

       if (ownerAccount == results.result) {
          console.log("correct owner: %s",ownerAccount);
          return cb();
       } else {
          return cb(new Error(util.format("incorrect owner: expected=%s, real=%s",ownerAccount,realOwner)));
       }
    });
    tx.on('error', function (err) {
        console.log("assign invoke error: %j", err);
        return cb(err);
    });
}

test('setup asset management with roles', function (t) {
    t.plan(1);

    console.log("setup asset management with roles");

    setup(function(err) {
        if (err) {
            t.fail("error: "+err.toString());
            process.exit(1);
        } else {
            t.pass("setup successful");
        }
    });
});

test('assign asset management with roles', function (t) {
    t.plan(1);
    alice.getUserCert(["role", "account"], function (err, aliceCert) {
        if (err) fail(t, "Failed getting Application certificate for Alice.");
        assignOwner(assigner, aliceCert.encode().toString('base64'), function(err) {
            if (err) {
                t.fail("error: "+err.toString());
            } else {
                checkOwner(assigner, aliceAccount, function(err) {
                    if(err){
                        t.fail("error: "+err.toString());
                    } else {
                        t.pass("assign successful");
                    }
                });
            }
        });
    })
});

test('not assign asset management with roles', function (t) {
    t.plan(1);

    bob.getUserCert(["role", "account"], function (err, bobCert) {
        if (err) fail(t, "Failed getting Application certificate for Alice.");
        assignOwner(alice, bobCert.encode().toString('base64'), function(err) {
            if (err) {
                t.fail("error: "+err.toString());
            } else {
                checkOwner(alice, bobAccount, function(err) {
                    if(err){
                        t.pass("assign successful");
                    } else {
                        err = new Error ("this user should not have been allowed to assign");
                        t.fail("error: "+err.toString());
                    }
                });
            }
        });
    });
});
