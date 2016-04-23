var hlc = require('../../hlc');
var test = require('tape');
var tapSpec = require('tap-spec');

//
//  Make test output pretty
//

test.createStream()
  .pipe(tapSpec())
  .pipe(process.stdout);

//
//  Configure a test chain
//

var chain = hlc.newChain("testChain");
chain.configureKeyValStore({dir:"/tmp/keyValStore"});
chain.setMemberServicesUrl("grpc://localhost:50051");


test('Set web app administrator', function(t) {
    t.plan(1);

    // Get the web app administrator member
    chain.getMember("myWebAppAdmin", function(err, webAppAdmin) {
    	if (err) {
        t.fail("Failed to get webAppAdmin member: " + err)
      }

      // Set the webAppAdmin as the designated chain registrar
    	chain.setRegistrar(webAppAdmin);

      // Check that the chain registrar is now webAppAdmin
      t.equal(chain.getRegistrar().getName(), 'myWebAppAdmin', 'Successfully set webAppAdmin!');
    });
});

test('Register and enroll user1', function(t) {
  t.plan(1);

  chain.getMember("user1",function(err,user) {
    if (err) {
      t.fail("Failed to get user1: " + err);
    }

    t.pass("Successfully registered and enrolled user1: " + user);
    });
});
