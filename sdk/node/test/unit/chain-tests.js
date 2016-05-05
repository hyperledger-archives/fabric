/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

var hlc = require('../../hlc');
var test = require('tape');
var util = require('util');
var fs = require('fs');

//
//  Create a test chain
//

var chain = hlc.newChain("testChain");

//
// Configure the test chain
//
// Set the directory for the local file-based key value store and point to the
// address of the membership service.
//

chain.configureKeyValStore({dir:"/tmp/keyValStore"});
chain.setMemberServicesUrl("grpc://localhost:50051");

//
// Enroll the WebAppAdmin member. WebAppAdmin member is already registered
// manually by being included inside the membersrvc.yaml file.
//

test('Enroll WebAppAdmin', function(t) {
    t.plan(2);

    // Get the WebAppAdmin member
    chain.getMember("WebAppAdmin", function(err, WebAppAdmin) {
    	if (err) {
        t.fail("Failed to get WebAppAdmin member " + " ---> " + err);
        t.end(err);
      } else {
        // Enroll the WebAppAdmin member with the certificate authority using
        // the one time password hard coded inside the membersrvc.yaml.
        pw = "DJY27pEnl16d";
        WebAppAdmin.enroll(pw, function(err, crypto) {
          if (err){
            t.fail("Failed to enroll WebAppAdmin member " + " ---> " + err);
            t.end(err);
          } else {
            t.pass("Successfully enrolled WebAppAdmin member" + " ---> " + JSON.stringify(crypto));

            // Confirm that the WebAppAdmin token has been created in the key value store
            path = chain.getKeyValStore().dir + "/member." + WebAppAdmin.getName();

            fs.exists(path, function(exists) {
              if (exists) {
                t.pass("Successfully stored client token for" + " ---> " + WebAppAdmin.getName());
              } else{
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

test('Set chain registrar', function(t) {
    t.plan(2);

    // Get the WebAppAdmin member
    chain.getMember("WebAppAdmin", function(err, WebAppAdmin) {
    	if (err) {
        t.fail("Failed to get WebAppAdmin member " + " ---> " + err);
        t.end(err);
      } else {
        t.pass("Successfully got WebAppAdmin member" + " ---> " + WebAppAdmin);

        // Set the WebAppAdmin as the designated chain registrar
        chain.setRegistrar(WebAppAdmin);

        // Confirm that the chain registrar is now WebAppAdmin
        t.equal(chain.getRegistrar().getName(), "WebAppAdmin", "Successfully set chain registrar to" + " ---> " + WebAppAdmin.getName());
      }
    });
});

//
// Register and enroll a new user with the certificate authority.
// This will be performed by the registrar member, WebAppAdmin.
//

test('Register and enroll a new user', function(t) {
  t.plan(2);

  // Crete a test_user
  test_user = {
    name: "WebApp_user1",
    role: 1,
    account: "bank_a",
    affiliation: "00001"
  };

  // Register and enroll test_user
  chain.getMember(test_user, function(err, user) {
    if (err) {
      t.fail("Failed to get " + test_user.name + " ---> " + err);
      t.end(err);
    } else {
      t.pass("Successfully registered and enrolled " + test_user.name + " ---> " + user);

      // Confirm that the user token has been created in the key value store
      path = chain.getKeyValStore().dir + "/member." + test_user.name;
      fs.exists(path, function(exists) {
        if (exists) {
          t.pass("Successfully stored client token for" + " ---> " + test_user.name);
        } else{
          t.fail("Failed to store client token for " + test_user.name + " ---> " + err);
        }
      });
    }
  });
});
