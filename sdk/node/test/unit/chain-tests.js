var hlc = require('../../hlc');
var test = require('tape');

//
//  Create a test chain
//

var chain = hlc.newChain("testChain");

//
// Configure the test chain
//
// Set the directory for the local file-based key value store and point to the
// address of membership service.
//

chain.configureKeyValStore({dir:"/tmp/keyValStore"});
chain.setMemberServicesUrl("grpc://localhost:50051");

//
// Register the myWebAppAdmin member, which will serve as the chain 'registrar'.
//

test('Set web app administrator', function(t) {
    t.plan(1);

    // Get the web app administrator member
    chain.getMember("myWebAppAdmin", function(err, webAppAdmin) {
    	if (err) {
        t.fail("Failed to get webAppAdmin member: " + err)
      } else {
        // Set the webAppAdmin as the designated chain registrar
        chain.setRegistrar(webAppAdmin);

        // Confirm that the chain registrar is now webAppAdmin
        t.equal(chain.getRegistrar().getName(), 'myWebAppAdmin', 'Successfully set webAppAdmin!');
      }
    });
});

//
// Register an additional user, 'user1', using the myWebAppAdmin 'registrar'
// member.
//

test('Register and enroll user1', function(t) {
  t.plan(1);

  chain.getMember("user1", function(err, user) {
    if (err) {
      t.fail("Failed to get user1: " + err);
    } else {
      t.pass("Successfully registered and enrolled user1: " + user);
    }
  });
});
