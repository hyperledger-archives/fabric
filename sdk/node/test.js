/**
 * Temporary tester as it is developed.
 */

var hlc = require(__dirname+'/hlc');

var chainMgr = hlc.NewChainMgr();
var chain = chainMgr.getChain("test1",true);
chain.configureWallet({dir:"/tmp/wallet"});
chain.setMemberServicesUrl("grpc://localhost:50051");
// Get the web app administrator member which is set as the chain registrar
// whose credentials are (or will be) used to authorize registering other web users.
chain.getMember("myWebAppAdmin",function(err,webAppAdmin) {
	if (err) return console.log("failed to get webAppAdmin member");
	// Assume the webAppAdmin has already been enrolled using the secret provided by member services administrator
	// webAppAdmin.enroll("webAppAdminSecret",function(err){...}
	chain.setRegistrarMember(webAppAdmin);
	chain.getMember("user1",function(err,user) {
		if (err) return console.log("can't get member: %j",err);
		console.log("got %s: %s",user.getName(),user);
		user.getTransactionContexts(function(err,resp) {
			console.log("getTransactionContexts results: %s: %s",err,resp);
		});
	});
});