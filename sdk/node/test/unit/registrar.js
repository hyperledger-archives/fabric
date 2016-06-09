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

var hlc = require('../..');
var test = require('tape');
var util = require('util');
var fs = require('fs');

var keyValStorePath = "/tmp/keyValStore"
var keyValStorePath2 = keyValStorePath + "2";

//
// Run the registrar test
//
test('registrar test', function (t) {
    registrarTest(function(err) {
        if (err) fail(t, "registrarTest", err);
        else pass(t, "registrarTest");
    });
});

//
// Run the registrar test
//
test('enroll again', function (t) {
    enrollAgain(function(err) {
        if (err) fail(t, "enrollAgain", err);
        else pass(t, "enrollAgain");
    });
});

// The registrar test
function registrarTest(cb) {
   console.log("testRegistrar");
   //
   // Create and configure the test chain
   //
   var chain = hlc.newChain("testChain");
   chain.setKeyValStore(hlc.newFileKeyValStore(keyValStorePath));
   chain.setMemberServicesUrl("grpc://localhost:50051");
   chain.enroll("admin", "Xurw3yU9zI0l", function (err, admin) {
      if (err) return cb(err);
      chain.setRegistrar(admin);
      // Register and enroll webAdmin
      registerAndEnroll("webAdmin", {roles:['client']}, chain, function(err,webAdmin) {
         if (err) return cb(err);
         chain.setRegistrar(webAdmin);
         registerAndEnroll("webUser", null, chain, function(err, webUser) {
            if (err) return cb(err);
            chain.setRegistrar(webUser);
            registerAndEnroll("webUser2", null, chain, function(err) {
               if (!err) return cb(Error("webUser should not be allowed to register a client"));
               return cb();
            });
         });
      });
   });
}

// Register and enroll user 'name' with registrar info 'registrar' for chain 'chain'
function registerAndEnroll(name, registrar, chain, cb) {
    console.log("registerAndEnroll %s",name);
    // User is not enrolled yet, so perform both registration and enrollment
    var registrationRequest = {
         enrollmentID: name,
         account: "bank_a",
         affiliation: "00001",
         registrar: registrar
    };
    chain.registerAndEnroll(registrationRequest,cb);
}

// Force the client to try to enroll admin again by creating a different chain
// This should fail.
function enrollAgain(cb) {
   console.log("enrollAgain");
   //
   // Remove the file-based keyValStore
   // Create and configure testChain2 so there is no shared state with testChain
   // This is necessary to start without a local cache.
   //
   fs.renameSync(keyValStorePath,keyValStorePath2);
   var chain = hlc.newChain("testChain2");
   chain.setKeyValStore(hlc.newFileKeyValStore('/tmp/keyValStore'));
   chain.setMemberServicesUrl("grpc://localhost:50051");
   chain.enroll("admin", "Xurw3yU9zI0l", function (err, admin) {
      rmdir(keyValStorePath);
      fs.renameSync(keyValStorePath2,keyValStorePath);
      if (!err) return cb(Error("admin should not be allowed to re-enroll"));
      return cb();
   });
}

function rmdir(path) {
  if( fs.existsSync(path) ) {
    fs.readdirSync(path).forEach(function(file,index){
      var curPath = path + "/" + file;
      if(fs.lstatSync(curPath).isDirectory()) { // recurse
        rmdir(curPath);
      } else { // delete file
        fs.unlinkSync(curPath);
      }
    });
    fs.rmdirSync(path);
  }
}

function pass(t, msg) {
    t.pass("Success: [" + msg + "]");
    t.end();
}

function fail(t, msg, err) {
    t.pass("Failure: [" + msg + "]: [" + err + "]");
    t.end(err);
}
