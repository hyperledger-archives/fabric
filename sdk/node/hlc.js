/**
 * "hlc" stands for "HyperLedger Client".
 * The Hyperledger Client SDK provides APIs through which a client can interact with a hyperledger blockchain.
 * 
 * Terminology:
 * 1) member - an identity for participating in the blockchain.  There are different types of members (users, peers, etc).
 * 2) member services - services related to obtaining and managing members
 * 3) registration - The act of adding a new member identity (with specific privileges) to the system.
 *               This is done by a member with the 'registrar' privilege.  The member is called a registrar.
 *               The registrar specifies the new member privileges when registering the new member.
 * 4) enrollment - Think of this as completing the registration process.  It may be done by the new member with a secret
 *               that it has obtained out-of-band from a registrar, or it may be performed by a middle-man who has
 *               delegated authority to act on behalf of the new member.
 * 
 * These APIs have been designed to support two pluggable components.
 * 1) Pluggable key value store which is used to retrieve and store keys associated with a member.
 *    Call Chain.setKeyValStore() to override the default key value store implementation.
 *    For the default implementations, see FileKeyValStore and SqlKeyValStore (TBD).
 * 2) Pluggable member service which is used to register and enroll members.
 *    Call Chain.setMemberService() to override the default implementation.
 *    For the default implementation, see MemberServices.
 *    NOTE: This makes member services pluggable from the client side, but more work is needed to make it compatible on
 *          the server side transaction processing path depending on how different the implementation is.
 */

var debug = require('debug')('hlc');   // 'hlc' stands for 'HyperLedger Client'
var fs = require('fs');
var urlParser = require("url");
var grpc = require("grpc");
var util = require('util');
//crypto stuff
var jsrsa = require('jsrsasign');
var KEYUTIL = jsrsa.KEYUTIL;
var asn1 = jsrsa.asn1;
var elliptic = require('elliptic');
var sha3_256 = require('js-sha3').sha3_256;
var sha3_384 = require('js-sha3').sha3_384;
var kdf = require(__dirname+'/kdf');

var _caProtos = grpc.load(__dirname + "/protos/ca.proto").protos;
var _timeStampProto = grpc.load(__dirname + "/protos/google/protobuf/timestamp.proto").google.protobuf.Timestamp;
var _chains = {};

/**
 * Create a new chain.  If it already exists, throws an Error. 
 * @param name {string} Name of the chain.  It can be any name and has value only for the client.
 * @returns
 */
function newChain(name) {
	var chain = _chains[name];
	if (chain) throw Error(util.format("chain %s already exists",name));
	chain = new Chain(name);
	_chains[name] = chain;
	return chain;
}

/**
 * Get a chain.  If it doesn't yet exist and 'create' is true, create it.
 * @param {string} chainName The name of the chain to get or create.
 * @param {boolean} create If the chain doesn't already exist, specifies whether to create it.
 * @return {Chain} Returns the chain, or null if it doesn't exist and create is false.
 */
function getChain(chainName, create) {
   var chain = _chains[chainName];
   if (!chain && create) {
	   chain = newChain(name);
   }
   return chain;
};

/**
 * Stop/cleanup everything pertaining to this module.
 */
function stop() {
   // Shutdown each chain
   for (var chainName in _chains) {
      _chains[chainName].shutdown();
   }
};

/**
 * The chain constructor.
 * @param {string} name The name of the chain.  This can be any name that the client chooses.
 * @returns {Chain} A chain client
 */
function Chain(name) {
   this.name = name;
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
 * Get the member whose credentials are used to register and enroll other users, or undefined if not set.
 * @param {Member} The member whose credentials are used to perform registration, or undefined if not set.
 */
Chain.prototype.getRegistrar = function() {
	return this.registrar;
};

/**
 * Set the member whose credentials are used to register and enroll other users.
 * @param {Member} registrar The member whose credentials are used to perform registration.
 */
Chain.prototype.setRegistrar = function(registrar) {
	this.registrar = registrar;
};

/**
 * Set the member services URL
 * @param {string} url Member services URL of the form: "grpc://host:port" or "grpcs://host:port"
 */
Chain.prototype.setMemberServicesUrl = function(url) {
	this.setMemberServices(newMemberServices(url));
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
 * Configure the key value store.
 * @param {Object} config Configuration for the default key value store of the form: TBD 
 */
Chain.prototype.configureKeyValStore  = function(config) {
	if (config.dir) {
		this.keyValStore = new FileKeyValStore(config.dir);
	} else {
		throw Error("invalid key value store config; no 'dir'");
	}
};

/**
 * Get the member services associated this chain.
 * @returns {MemberServices} Return the current member services, or undefined if not set.
 */
Chain.prototype.getKeyValStore  = function() {
   return this.keyValStore;
};

/**
 * Set the key value store.  This allows the default implementation of the key value store to be overridden.
 */
Chain.prototype.setKeyValStore  = function(keyValStore) {
   this.keyValStore = keyValStore;
};

/**
 * Get the user member named 'name'.
 * @param cb Callback of signature "function(err,Member)"
 */
Chain.prototype.getMember = function(name,cb) {
	var self = this;
	cb = cb || nullCB;
	if (!self.keyValStore) return cb(Error("No key value store was found.  You must first call Chain.configureKeyValStore or Chain.setKeyValStore"));
	if (!self.memberServices) return cb(Error("No member services was found.  You must first call Chain.configureMemberServices or Chain.setMemberServices"));
	self._getMember(name,function(err,member) {
		if (err) return cb(err);
		if (!self.registrar) return cb(null,member);
		member.registerAndEnroll(self.registrar,function(err) {
			if (err) return cb(err);
			cb(null,member);
		});
	});
};

Chain.prototype._getMember = function(name,cb) {
	var self = this;
	// Try to get the member state from the cache
	var member = self.members[name];
	if (member) return cb(null,member);
	// Create the member and try to restore it's state from the key value store (if found).
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
	this.keyValStore = chain.getKeyValStore();
	this.keyValStoreName = toKeyValStoreName(name);
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
Member.prototype.register = function(registrar,cb) {
	var self = this;
	cb = cb || nullCB;
	var enrollmentSecret = this.state.enrollmentSecret;
	if (enrollmentSecret) {
		debug("previously registered, enrollmentSecret=%s",enrollmentSecret);
		return cb(null,enrollmentSecret);
	}
	self.memberServices.register( {name:self.getName()}, function(err,enrollmentSecret) {
		debug("memberServices.register err=%s, secret=%s",err,enrollmentSecret);
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
	var enrollment = self.state.enrollment;
	if (enrollment) {
		debug("previously enrolled, enrollment=%j",enrollment);
		return cb(null,enrollment);
	}
	var req = {name: self.getName(), enrollmentSecret: enrollmentSecret};
	self.memberServices.enroll( req, function(err,enrollment) {
		debug("memberServices.enroll err=%s, enrollment=%j",err,enrollment);
		if (err) return cb(err);
		self.state.enrollment = enrollment;
		self.saveState(function(err) {
			if (err) return cb(err);
			cb(null,enrollment);
		});
	});
};

/**
 * Perform both registration and enrollment.
 * @param cb Callback of the form: {function(err,{key,cert,chainKey})}
 */
Member.prototype.registerAndEnroll = function(registrar,cb) {
	var self = this;
	cb = cb || nullCB;
	var enrollment = self.state.enrollment;
	if (enrollment) {
		debug("previously enrolled, enrollment=%j",enrollment);
		return cb(null,enrollment);
	}
	self.register(registrar, function(err,enrollmentSecret) {
		if (err) return cb(err);
		self.enroll(enrollmentSecret, cb);
	});
};

/**
 * Get a transaction contexts.
 * @param anonymousMode Get a transaction manager which will perform transactions anonymously.
 * @return {TransactionMgr} A transaction manager.
 */
Member.prototype.getTransactionContexts = function(cb) {
	var self = this;
	self.memberServices.getTransactionContexts({name:self.getName(),enrollment:self.state.enrollment,num:1}, function(err,resp) {
		if (err) return cb(err);
		cb(resp);
	});
};

/**
 * Save the state of this member to the key value store.
 * @param cb Callback of the form: {function(err}
 */
Member.prototype.saveState = function(cb) {
	var self = this;
	self.keyValStore.setValue(self.keyValStoreName,self.toString(),cb);
};

/**
 * Restore the state of this member from the key value store (if found).  If not found, do nothing.
 * @param cb Callback of the form: {function(err}
 */
Member.prototype.restoreState = function(cb) {
	var self = this;
	self.keyValStore.getValue(self.keyValStoreName, function(err,memberStr) {
		if (err) return cb(err);
		debug("restoreState: name=%s, memberStr=%s",self.getName(),memberStr);
		if (memberStr) {
			// The member was found in the key value store, so restore the state.
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
    this.ecaaClient = new _caProtos.ECAA(addr,creds);
    this.ecapClient = new _caProtos.ECAP(addr,creds);
    this.tcapClient = new _caProtos.TCAP(addr,creds);
    this.tlscapClient = new _caProtos.TLSCAP(addr,creds);
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
    var protoReq = new _caProtos.RegisterUserReq();
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
    var eCertCreateRequest = new _caProtos.ECertCreateReq();
    var timestamp = new _timeStampProto({ seconds: Date.now() / 1000, nanos: 0 });
    eCertCreateRequest.setTs(timestamp);
    eCertCreateRequest.setId({ id: req.name });
    eCertCreateRequest.setTok({ tok: new Buffer(req.enrollmentSecret) });

    // public signing key (ecdsa)
    var signPubKey = new _caProtos.PublicKey(
        {
            type: _caProtos.CryptoType.ECDSA,
            key: new Buffer(spki.getASN1Object().getEncodedHex(), 'hex')
        });
    eCertCreateRequest.setSign(signPubKey);
    
    // public encryption key (ecdsa)
    var encPubKey = new _caProtos.PublicKey(
        {
            type: _caProtos.CryptoType.ECDSA,
            key: new Buffer(spki2.getASN1Object().getEncodedHex(), 'hex')
        });
    eCertCreateRequest.setEnc(encPubKey);

    self.ecapClient.createCertificatePair(eCertCreateRequest, function (err, eCertCreateResp) {
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

        eCertCreateRequest.setSig(new _caProtos.Signature(
            {
                type: _caProtos.CryptoType.ECDSA,
                r: new Buffer(sig.r.toString()),
                s: new Buffer(sig.s.toString())
            }
            ));
        self.ecapClient.createCertificatePair(eCertCreateRequest, function (err, eCertCreateResp) {
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

/**
 * Generically this gets an array of opaque transaction context objects, where each transaction context
 * can be passed into the sign and encrypt functions of MemberServices.
 * For default member services, each transaction context is a tcert.
 * @param {Object} req Request of the form: {name,enrollment,num} where
 * 'name' is the member name,
 * 'enrollment' is what was returned by enroll, and
 * 'num' is the number of transaction contexts to obtain.
 * @param {function(err,[Object])} cb The callback function which is called with an error as 1st arg and an array of opaque transaction contexts as 2nd arg.
 */
MemberServices.prototype.getTransactionContexts = function (req, cb) {
    var self = this;
    cb = cb || nullCB;

    var timestamp = new _timeStampProto({ seconds: Date.now() / 1000, nanos: 0 });
    
    // create the proto
    var tCertCreateSetReq = new _caProtos.TCertCreateSetReq();
    tCertCreateSetReq.setTs(timestamp);
    tCertCreateSetReq.setId({ id: req.name });
    tCertCreateSetReq.setNum(req.num);
    
    // serialize proto
    var buf = tCertCreateSetReq.toBuffer();
    
    // sign the transaction using enrollment key
    var EC = elliptic.ec;
    var curve = elliptic.curves.p384;
    var ecdsa = new EC(curve);
    var signKey = ecdsa.keyFromPrivate(req.enrollment.key, 'hex');
    // debug(new Buffer(sha3_384(buf),'hex'));
    var sig = ecdsa.sign(new Buffer(sha3_256(buf), 'hex'), signKey);

    tCertCreateSetReq.setSig(new _caProtos.Signature(
        {
            type: _caProtos.CryptoType.ECDSA,
            r: new Buffer(sig.r.toString()),
            s: new Buffer(sig.s.toString())
        }
        ));

    // send the request
    self.tcapClient.createCertificateSet(tCertCreateSetReq, function (err, tCertCreateSetResp) {
        if (err) return cb(err);
        debug('tCertCreateSetResp:\n', tCertCreateSetResp);
        cb(null, tCertCreateSetResp.certs);
    });

};

function newMemberServices(url) {
	return new MemberServices(url);
}

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
 * A local file-based key value store.
 */
function FileKeyValStore(dir) {
	this.dir = dir;
	if (!fs.existsSync(dir)) {
	    fs.mkdirSync(dir);
	}
}

/**
 * Get the value associated with name.
 * @param name
 * @param cb function(err,value)
 */
FileKeyValStore.prototype.getValue = function(name,cb) {
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
FileKeyValStore.prototype.setValue = function(name,value,cb) {
	var path = this.dir + '/' + name;
	fs.writeFile(path,value,cb);
};

function toKeyValStoreName(name) {
   return "member."+name;	
}

/**
 * Create and load peers for bluemix.
 */
function bluemixInit() {
   var vcap = process.env.VCAP_SERVICES;
   if (!vcap) return false; // not in bluemix
   // TODO: Pilfer logic from marbles app   
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

exports.newChain = newChain;
exports.getChain = getChain;
exports.newMemberServices = newMemberServices;
exports.bluemixInit = bluemixInit;