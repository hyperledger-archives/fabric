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

var chain, chaincodeID;
var chaincodeName = "assetmgmt_with_roles";
var deployer, assigner, nonAssigner;
var networkMode = process.env.NETWORK_MODE;

// Create the chain and enroll users as deployer, assigner, and nonAssigner (who doesn't have privilege to assign.
function setup(cb) {
   console.log("initializing ...");
   var chain = hlc.newChain("testChain");
   chain.setKeyValStore(hlc.newFileKeyValStore("/tmp/keyValStore"));
   chain.setMemberServicesUrl("grpc://localhost:50051");
   chain.addPeer("grpc://localhost:30303");
   console.log("enrolling deployer ...");
   chain.enroll("WebAppAdmin", "DJY27pEnl16d", function (err, user) {
      if (err) return cb(err);
      deployer = user;
      console.log("enrolling assigner ...");
      chain.enroll("jim","6avZQLwcUe9b", function (err, user) {
         if (err) return cb(err);
         assigner = user;
         console.log("enrolling nonAssigner ...");
         chain.enroll("lukas","NPKYL39uKbkj", function (err, user) {
            if (err) return cb(err);
            nonAssigner = user;
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
        metadata: "assigner"
    };
    if (networkMode) {
       req.chaincodePath = "github.com/asset_management_with_roles/";
    } else {
       req.chaincodeName = "asset_management_with_roles";
    }
    var tx = admin.deploy(req);
    tx.on('submitted', function (results) {
        console.log("deploy submitted: %j", results);
    });
    tx.on('complete', function (results) {
        console.log("deploy complete: %j", results);
        chaincodeID = results.chaincodeID;
        return assign(cb);
    });
    tx.on('error', function (err) {
        console.log("deploy error: %j", err.toString());
        return cb(err);
    });
}

function assign(user,cb) {
    var req = {
        chaincodeID: chaincodeID,
        fcn: "assign",
        args: ["MyAsset","ownerID"],
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

test('setup asset management with roles', function (t) {
    setup(function(err) {
        if (err) {
            t.fail("error: "+err.toString());
            t.end(err);
            process.exit(1);
        } else {
            t.pass("setup successful");
            t.end();
        }
    });
});

test('deploy asset management with roles', function (t) {
    deploy(function(err) {
        if (err) {
            t.fail("error: "+err.toString());
            t.end(err);
            process.exit(1);
        } else {
            t.pass("deploy successful");
            t.end();
        }
    });
});

test('assign asset management with roles', function (t) {
    assign(assigner, function(err) {
        if (err) {
            t.fail("error: "+err.toString());
            t.end(err);
        } else {
            t.pass("assign successful");
            t.end();
        }
    });
});

test('not assign asset management with roles', function (t) {
    assign(notAssigner, function(err) {
        if (err) {
            t.pass("not assign successful");
            t.end();
        } else {
            err = new Error ("this user should not have been allowed to assign");
            t.fail("error: "+err.toString());
            t.end(err);
        }
    });
});
