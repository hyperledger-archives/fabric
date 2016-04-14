/**
 * The Hyperledger client SDK provides APIs through which a client can interact with an existing hyperledger blockchain.
 * 
 * These APIs have been designed to support two pluggable components.
 * 1) Pluggable wallet which is used to retrieve and store keys associated with a member.
 *    Call Chain.setWallet() to override the default wallet implementation.
 *    For the default implementations, see FileWallet and SqlWallet (TBD).
 * 2) Pluggable member services which is used to register and enroll members.
 *    Call Chain.setMemberService() to override the default implementation.
 *    For the default implementation, see MemberServices.
 *    NOTE: This makes member services pluggable from the client side, but more work is needed to make it compatible on
 *          the server side transaction processing path depending on how different the implementation is.
 */

var debug = require('debug')('hlc');   // 'hlc' stands for 'HyperLedger Client'
var fs = require('fs');
var urlParser = require("url");
var grpc = require("grpc");
//crypto stuff
var jsrsa = require('jsrsasign');
var KEYUTIL = jsrsa.KEYUTIL;
var asn1 = jsrsa.asn1;
var elliptic = require('elliptic');
var sha3_256 = require('js-sha3').sha3_256;
var sha3_384 = require('js-sha3').sha3_384;
var kdf = require(__dirname+'/kdf');

var caProtos = grpc.load(__dirname + "/protos/ca.proto").protos;
var timeStampProto = grpc.load(__dirname + "/protos/google/protobuf/timestamp.proto").google.protobuf.Timestamp;

function NewChainMgr() {
	return new ChainMgr();
}

function NewMemberServices(url) {
	return new MemberServices(url);
}

exports.NewChainMgr = NewChainMgr;
exports.NewMemberServices = NewMemberServices;

/**
 * Constructor for the chain client manager to manage one or more chain clients
 */
function ChainMgr() {
   this.chains = {};
   this.defaultTimeout = 60 * 1000;  // 1 minute
   // If running in bluemix, initialize from VCAP_SERVICES environment variable
   if (process.env.VCAP_SERVICES) {
      // TODO: From marbles app   
   }
}

/**
 * Get a chain.  If it doesn't yet exist and 'create' is true, create it.
 * @param {string} chainName The name of the chain to get or create.
 * @param {boolean} create If the chain doesn't already exist, specifies whether to create it.
 * @return {Chain} Returns the chain, or null if it doesn't exist and create is false.
 */
ChainMgr.prototype.getChain = function (chainName, create) {
   var chain = this.chains[chainName];
   if (!chain && create) {
      chain = new Chain(chainName,this);
      this.chains[chainName] = chain;
   }
   return chain;
};

/**
 * Get the default timeout in milliseconds.
 * @return {int} Returns the default timeout in milliseconds.
 */
ChainMgr.prototype.getDefaultTimeoutInMs = function () {
   return this.timeout;
};

/**
 * Set the default timeout in milliseconds.
 * @param {int} The default timeout milliseconds.
 */
ChainMgr.prototype.setDefaultTimeoutInMs = function (timeout) {
   this.timeout = timeout;
};

/**
 * Shutdown/cleanup everything related to the chain manager
 */
ChainMgr.prototype.shutdown = function () {
   var self = this;
   // Shutdown each chain
   for (var chainName in self.chains) {
      self.chains[chainName].shutdown();
   }
};

/**
 * The chain constructor.
 * @param {string} name The name of the chain.  This can be any name that the client chooses.
 * @param {ChainMgr} mgr The manager used to create this chain.
 * @returns {Chain} A chain client
 */
function Chain(name,mgr) {
   this.name = name;
   this.mgr = mgr;
   this.peers = [];
   this.members = {};  // TODO: Make this an LRU cache to limit the number of members cached in memory
}

/**
 * Get the chain name.
 * @returns {string} The name of the chain.
 */
Chain.prototype.getName = function() {
   return this.name;
};

/**
 * Get the chain manager
 * @returns {ChainMgr} The manager used to create this chain.
 */
Chain.prototype.getChainMgr = function() {
   return this.mgr;
};

/**
 * Add a peer given an endpoint specification.
 * @param {Object} endpoint The endpoint of the form: { url: "grpcs://host:port", tls: { .... } }
 * TBD: The format of 'endpoint.tls' depends upon the format expected by node's grpc module.  We may want to support Buffer and file path for convenience.
 * @returns {Peer} Returns a new peer.
 */
Chain.prototype.addPeer = function(endpoint) {
   var peer = new Peer(endpoint);
   this.peers.add(peer);
   return peer;
};

/**
 * Get the peers for this chain.
 */
Chain.prototype.getPeers  = function() {
   return this.peers;
};

/**
 * Set the member services URL
 * @param {string} url Member services URL of the form: "grpc://host:port" or "grpcs://host:port"
 */
Chain.prototype.setMemberServicesUrl = function(url) {
	this.setMemberServices(NewMemberServices(url));
};

/**
 * Get the member service associated this chain.
 * @returns {MemberService} Return the current member service, or undefined if not set.
 */
Chain.prototype.getMemberServices = function() {
   return this.memberServices;
};

/**
 * Set the member service associated this chain.  This allows the default implementation of member service to be overridden.
 */
Chain.prototype.setMemberServices = function(memberServices) {
   this.memberServices = memberServices;
};

/**
 * Configure the wallet.
 * @param {Object} config Configuration for the default wallet of the form: TBD 
 */
Chain.prototype.configureWallet  = function(config) {
	if (config.dir) {
		this.wallet = new FileWallet(config.dir);
	} else {
		throw Error("invalid wallet config; no 'dir'");
	}
};

/**
 * Get the member services associated this chain.
 * @returns {MemberServices} Return the current member services, or undefined if not set.
 */
Chain.prototype.getWallet  = function() {
   return this.wallet;
};

/**
 * Set the wallet.  This allows the default implementation of the wallet to be overridden.
 */
Chain.prototype.setWallet  = function(wallet) {
   this.wallet = wallet;
};

/**
 * Get the user member named 'name'.
 * @param cb Callback of signature "function(err,Member)"
 */
Chain.prototype.getMember = function(name,cb) {
	var self = this;
	cb = cb || nullCB;
	// Try to get the member state from the cache
	var member = self.members[name];
	if (member) return cb(null,member);
	// Not found in the cache.  Need to make sure there is a wallet and member services for this chain.
	if (!self.wallet) return cb(Error("No wallet was found.  You must first call Chain.configureWallet or Chain.setWallet"));
	if (!self.memberServices) return cb(Error("No member services was found.  You must first call Chain.configureMemberServices or Chain.setMemberServices"));
	// Create the member and try to restore it's state from the wallet (if found).
	member = new Member(name,this);
	member.restoreState(function(err) {
		if (err) return cb(err);
		cb(null,member);
	});
};

/**
 * Constructor for a member.
 * @param name The member name.
 * @returns {Member} A member who is neither registered nor enrolled.
 */
function Member(name,chain) {
	this.state = {name:name};
	this.chain = chain;
	this.memberServices = chain.getMemberServices();
	this.wallet = chain.getWallet();
	this.walletName = toWalletName(name);
}

/**
 * Get the member name.
 * @returns {string} The member name.
 */
Member.prototype.getName = function() {
	return this.state.name;
};

/**
 * Determine if this name has been registered.
 * @returns {boolean} True if registered; otherwise, false.
 */
Member.prototype.isRegistered = function() {
	return this.isEnrolled() || this.state.enrollmentKey;
};

/**
 * Determine if this name has been enrolled.
 * @returns {boolean} True if enrolled; otherwise, false.
 */
Member.prototype.isEnrolled = function() {
	return this.state.enrollment;
};

/**
 * Register the member.
 * @param req Registration request with the following fields: name, role 
 * @param cb Callback of the form: {function(err,enrollmentSecret)}
 */
Member.prototype.register = function(registrarMember,cb) {
	var self = this;
	cb = cb || nullCB;
	if (self.isRegistered()) return cb(Error(self.getName()+" is already registered"));
	self.memberServices.register( {name:self.getName()}, function(err,enrollmentSecret) {
		if (err) return cb(err);
		self.state.enrollmentSecret = enrollmentSecret;
		self.saveState(function(err) {
			if (err) return cb(err);
			cb(null,enrollmentSecret);
		});
	});
};

/**
 * Enroll the member and return the enrollment results.
 * @param enrollmentSecret The enrollment secret as returned by register.
 * @param cb Callback of the form: {function(err,{key,cert,chainKey})}
 */
Member.prototype.enroll = function(enrollmentSecret, cb) {
	var self = this;
	cb = cb || nullCB;
	if (self.isEnrolled()) return cb(Error(self.getName()+" is already enrolled"));
	var req = {name: self.getName(), enrollmentSecret: enrollmentSecret};
	self.memberServices.enroll( req, function(err,enrollment) {
		if (err) return cb(err);
		self.state.enrollment = enrollment;
		cb(null,enrollment);
	});
};

/**
 * Perform both registration and enrollment.
 * @param cb Callback of the form: {function(err,{key,cert,chainKey})}
 */
Member.prototype.registerAndEnroll = function(registrarMember,cb) {
	var self = this;
	cb = cb || nullCB;
	self.register(registrarMember, function(err,enrollmentSecret) {
		if (err) return cb(err);
		self.enroll(enrollmentSecret, cb);
	});
};

/**
 * Get a transaction manager.
 * @param anonymousMode Get a transaction manager which will perform transactions anonymously.
 * @return {TransactionMgr} A transaction manager.
 */
Member.prototype.getTransactionMgr = function(anonymousMode) {
	return new TransactionMgrImpl(this);
};

/**
 * Save the state of this member to the wallet.
 * @param cb Callback of the form: {function(err}
 */
Member.prototype.saveState = function(cb) {
	var self = this;
	self.wallet.setValue(self.walletName,self.toString(),cb);
};

/**
 * Restore the state of this member from the wallet, if found in the wallet.
 * @param cb Callback of the form: {function(err}
 */
Member.prototype.restoreState = function(cb) {
	var self = this;
	self.wallet.getValue(self.walletName, function(err,memberStr) {
		if (err) return cb(err);
		debug("restoreState: name=%s, memberStr=%s",self.getName(),memberStr);
		if (memberStr) {
			// The member was found in the wallet, so restore the state.
			self.fromString(memberStr);
		}
		cb();
	});
};

/**
 * Save the current state of this member as a string
 * @return {string} The state of this member as a string
 */
Member.prototype.fromString = function(str) {
	var state = JSON.parse(str);
	if (state.name !== this.getName()) throw Error("name mismatch: '"+state.name+"' does not equal '"+this.getName()+"'");
	this.state = state;
};

/**
 * Save the current state of this member as a string
 * @return {string} The state of this member as a string
 */
Member.prototype.toString = function() {
	return JSON.stringify(this.state);
};

/**
 * Constructor for a peer given the endpoint config for the peer.
 * @param {Object} config The endpoint config of the form: { url: "grpcs://host:port", tls: { .... } }
 * TBD: The format of 'config.tls' depends upon the format expected by node's grpc module.
 * @param {Chain} The chain of which this peer is a member.
 * @returns {Peer} The new peer.
 */
function Peer(endpoint,chain) {
   this.endpoint = endpoint;
   this.chain = chain;
}

/**
 * Get the chain of which this peer is a member.
 * @returns {Chain} The chain of which this peer is a member.
 */
Peer.prototype.getChain = function() {
   return this.chain;
};

/**
 * Get the URL of the peer.
 * @returns {string} Get the URL associated with the peer.
 */
Peer.prototype.getUrl = function() {

};

/**
 * Remove the peer from the chain.
 */
Peer.prototype.remove = function() {
   throw Error("TODO: implement");
};

/**
 * MemberServices constructor
 * @param config The config information required by this member services implementation.
 * @returns {MemberServices} A MemberServices object.
 */
function MemberServices(url) {
	var purl = parseUrl(url);
	var protocol = purl.protocol.toLowerCase();
	var addr, creds;
	if (protocol === 'grpc') {
		addr = purl.host;
		creds = grpc.credentials.createInsecure();
	} else if (protocol === 'grpcs') {
		addr = purl.host;
		creds = grpc.credentials.createSsl();
	} else {
		throw Error("invalid protocol: "+protocol);
	}
    this.ecaaClient = new caProtos.ECAA(addr,creds);
    this.ecapClient = new caProtos.ECAP(addr,creds);
    this.tcapClient = new caProtos.TCAP(addr,creds);
    this.tlscapClient = new caProtos.TLSCAP(addr,creds);
}

/**
 * Register the member and return an enrollment secret.
 * @param req Registration request with the following fields: name, role 
 * @param cb Callback of the form: {function(err,enrollmentSecret)}
 */
MemberServices.prototype.register = function(req,cb) {
	var self = this;
	if (!req.name) return cb(new Error("missing req.name"));
	var role = req.role || 1;
    var protoReq = new caProtos.RegisterUserReq();
    protoReq.setId({ id: req.name });
    protoReq.setRole(role);
    self.ecaaClient.registerUser(protoReq, function (err, token) {
    	debug("register %j: err=%j, token=%s",protoReq,err,token);
    	if (cb) return cb(err,token?token.tok.toString():null);
    });
};

/**
 * Enroll the member and return an opaque member object
 * @param req Enrollment request with the following fields: name, enrollmentSecret
 * @param cb Callback of the form: {function(err,{key,cert,chainKey})}
 */
MemberServices.prototype.enroll = function (req, cb) {
    var self = this;
    cb = cb || nullCB;

    if (!req.name) return cb(Error("req.name is not set"));
    if (!req.enrollmentSecret) return cb(Error("req.enrollmentSecret is not set"));

    // generate ECDSA keys: signing and encryption keys
    // 1) signing key
    var ecKeypair = KEYUTIL.generateKeypair("EC", "secp384r1");
    var spki = new asn1.x509.SubjectPublicKeyInfo(ecKeypair.pubKeyObj);
    // 2) encryption key
    var ecKeypair2 = KEYUTIL.generateKeypair("EC", "secp384r1");
    var spki2 = new asn1.x509.SubjectPublicKeyInfo(ecKeypair2.pubKeyObj);
    
    // create the proto message
    var eCertCreateRequest = new caProtos.ECertCreateReq();
    var timestamp = new timeStampProto({ seconds: Date.now() / 1000, nanos: 0 });
    eCertCreateRequest.setTs(timestamp);
    eCertCreateRequest.setId({ id: req.name });
    eCertCreateRequest.setTok({ tok: new Buffer(req.enrollmentSecret) });

    // public signing key (ecdsa)
    var signPubKey = new caProtos.PublicKey(
        {
            type: caProtos.CryptoType.ECDSA,
            key: new Buffer(spki.getASN1Object().getEncodedHex(), 'hex')
        });
    eCertCreateRequest.setSign(signPubKey);
    
    // public encryption key (ecdsa)
    var encPubKey = new caProtos.PublicKey(
        {
            type: caProtos.CryptoType.ECDSA,
            key: new Buffer(spki2.getASN1Object().getEncodedHex(), 'hex')
        });
    eCertCreateRequest.setEnc(encPubKey);

    self.createCertificatePair(eCertCreateRequest, function (err, eCertCreateResp) {
        if (err) return cb(err);
        var cipherText = eCertCreateResp.tok.tok;
        // cipherText = ephemeralPubKeyBytes + encryptedTokBytes + macBytes
        // ephemeralPubKeyBytes = first ((384+7)/8)*2 + 1 bytes = first 97 bytes
        // hmac is sha3_384 = 48 bytes or sha3_256 = 32 bytes
        var ephemeralPublicKeyBytes = cipherText.slice(0, 97);
        var encryptedTokBytes = cipherText.slice(97, cipherText.length - 32);
        debug("encryptedTokBytes:\n", encryptedTokBytes);
        var macBytes = cipherText.slice(cipherText.length - 48);
        debug("length = ", ephemeralPublicKeyBytes.length + encryptedTokBytes.length + macBytes.length);
        //debug(rsaPrivKey.decrypt(eCertCreateResp.tok.tok));
        debug('encrypted Tok: ', eCertCreateResp.tok.tok);
        debug('encrypted Tok length: ', eCertCreateResp.tok.tok.length);
        //debug('public key obj:\n',ecKeypair2.pubKeyObj);
        debug('public key length: ', new Buffer(ecKeypair2.pubKeyObj.pubKeyHex, 'hex').length);
        //debug('private key obj:\n',ecKeypair2.prvKeyObj);
        debug('private key length: ', new Buffer(ecKeypair2.prvKeyObj.prvKeyHex, 'hex').length);

        var EC = elliptic.ec;
        var curve = elliptic.curves.p384;
        var ecdsa = new EC(curve);
        
        // convert bytes to usable key object
        var ephPubKey = ecdsa.keyFromPublic(ephemeralPublicKeyBytes.toString('hex'), 'hex');
        var encPrivKey = ecdsa.keyFromPrivate(ecKeypair2.prvKeyObj.prvKeyHex, 'hex');

        var secret = encPrivKey.derive(ephPubKey.getPublic());
        var aesKey = kdf.hkdf(secret.toArray(), 256, null, null, 'sha3-256');

        // debug('aesKey: ',aesKey);
        
        var decryptedTokBytes = kdf.aesCFBDecryt(aesKey, encryptedTokBytes);
        
        // debug(decryptedTokBytes);
        debug(decryptedTokBytes.toString());

        eCertCreateRequest.setTok({ tok: decryptedTokBytes });
        eCertCreateRequest.setSig(null);

        var buf = eCertCreateRequest.toBuffer();

        var signKey = ecdsa.keyFromPrivate(ecKeypair.prvKeyObj.prvKeyHex, 'hex');
        // debug(new Buffer(sha3_384(buf),'hex'));
        var sig = ecdsa.sign(new Buffer(sha3_256(buf), 'hex'), signKey);

        eCertCreateRequest.setSig(new caProtos.Signature(
            {
                type: caProtos.CryptoType.ECDSA,
                r: new Buffer(sig.r.toString()),
                s: new Buffer(sig.s.toString())
            }
            ));
        self.createCertificatePair(eCertCreateRequest, function (err, eCertCreateResp) {
            if (err) return cb(err);
            debug(eCertCreateResp);
            this.enroll = {
               key: ecKeypair.prvKeyObj.prvKeyHex,
               cert: eCertCreateResp.certs.sign.toString('hex'),
               chainKey: eCertCreateResp.pkchain.toString('hex')
            };
            // debug('cert:\n\n',this.enroll.cert)
            cb(null,this.enroll);
        });
    });

};

MemberServices.prototype.createCertificatePair = function (eCertCreateRequest, cb) {
    this.ecapClient.createCertificatePair(eCertCreateRequest, function (err, eCertCreateResp) {
        if (err) {
            debug('error:\n', err);
            return cb(err);
        }
        cb(null, eCertCreateResp);
    });
};

/**
 * Constructor for a transaction manager.
 * By default, make both anonymous and private.
 */
function TransactionMgrImpl(member) {
	this.member = member;
	this.setAnonymous(true);
	this.setPrivate(true);
}

TransactionMgrImpl.prototype.isAnonymous = function() {
	return this.anonymous;
};

TransactionMgrImpl.prototype.setAnonymous = function(anonymous) {
	this.anonymous = true;
};

TransactionMgrImpl.prototype.isPrivate = function() {
	return this.privateMode;
};

TransactionMgrImpl.prototype.setPrivate = function(privateMode) {
	this.privateMode = privateMode;
};

/**
 * A local file-based wallet.
 */
function FileWallet(dir) {
	this.dir = dir;
}

/**
 * Get the value associated with name.
 * @param name
 * @param cb function(err,value)
 */
FileWallet.prototype.getValue = function(name,cb) {
	var path = this.dir + '/' + name;
	fs.readFile(path,'utf8',function(err,data) {
		if (err) {
			if (err.code !== 'ENOENT') return cb(err);
			return cb();
		}
		return cb(null,data);
	});
};

/**
 * Set the value associated with name.
 * @param name
 * @param cb function(err)
 */
FileWallet.prototype.setValue = function(name,value,cb) {
	var path = this.dir + '/' + name;
	fs.writeFile(path,value,cb);
};

function toWalletName(name) {
   return "member."+name;	
}

function nullCB() {}

// Determine if an object is a string
function isString(obj) {
   return (typeof obj === 'string' || obj instanceof String);
}

// Determine if 'obj' is an object (not an array, string, or other type)
function isObject(obj) {
   return (!!obj) && (obj.constructor === Object);
}

function isFunction(fcn) {
  return (typeof fcn === 'function');
}

function parseUrl(url) {
   var purl = urlParser.parse(url,true);
   if (purl.protocol.endsWith(":")) {
	   purl.protocol = purl.protocol.slice(0,-1);
   }
   return purl;
}

// Define startsWith method for String object for convenience
if(!String.prototype.startsWith) {
    String.prototype.startsWith = function (str) {
        return !this.indexOf(str);
    };
}

// Define endsWith method for String object for convenience
if(!String.prototype.endsWith) {
    String.prototype.endsWith = function (s) {
        return this.length >= s.length && this.substr(this.length - s.length) === s;
    };
}


function test() {
	var enrollSecret = "vgPnEsLebTxo";
	var chainMgr = NewChainMgr();
	var chain = chainMgr.getChain("test1",true);
	chain.configureWallet({dir:"/tmp/wallet"});
	chain.setMemberServicesUrl("grpc://localhost:50051");
	chain.getMember("user3",function(err,member) {
		if (err) return console.log("can't get member: %s",err);
		if (!member.isRegistered()) {
			debug("enrolling");
			member.enroll(enrollSecret,function(err) {
				debug("enrolled: err="+err);
			});
		} else {
			debug("already enrolled");
		}
	});
}

test();
