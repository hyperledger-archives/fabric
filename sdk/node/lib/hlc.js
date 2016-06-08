"use strict";
var __extends = (this && this.__extends) || function (d, b) {
    for (var p in b) if (b.hasOwnProperty(p)) d[p] = b[p];
    function __() { this.constructor = d; }
    d.prototype = b === null ? Object.create(b) : (__.prototype = b.prototype, new __());
};
process.env['GRPC_SSL_CIPHER_SUITES'] = 'HIGH+ECDSA';
var debugModule = require('debug');
var fs = require('fs');
var targz = require('tar.gz');
var urlParser = require('url');
var grpc = require('grpc');
var util = require('util');
var jsrsa = require('jsrsasign');
var elliptic = require('elliptic');
var sha3 = require('js-sha3');
var uuid = require('node-uuid');
var BN = require('bn.js');
var crypto = require("./crypto");
var stats = require("./stats");
var sdk_util = require("./sdk_util");
var events = require('events');
var debug = debugModule('hlc');
var asn1 = jsrsa.asn1;
var asn1Builder = require('asn1');
var _caProto = grpc.load(__dirname + "/protos/ca.proto").protos;
var _fabricProto = grpc.load(__dirname + "/protos/fabric.proto").protos;
var _timeStampProto = grpc.load(__dirname + "/protos/google/protobuf/timestamp.proto").google.protobuf.Timestamp;
var _chaincodeProto = grpc.load(__dirname + "/protos/chaincode.proto").protos;
var net = require('net');
var DEFAULT_SECURITY_LEVEL = 256;
var DEFAULT_HASH_ALGORITHM = "SHA3";
var CONFIDENTIALITY_1_2_STATE_KD_C6 = 6;
var _chains = {};
(function (PrivacyLevel) {
    PrivacyLevel[PrivacyLevel["Nominal"] = 0] = "Nominal";
    PrivacyLevel[PrivacyLevel["Anonymous"] = 1] = "Anonymous";
})(exports.PrivacyLevel || (exports.PrivacyLevel = {}));
var PrivacyLevel = exports.PrivacyLevel;
var Certificate = (function () {
    function Certificate(cert, privateKey, privLevel) {
        this.cert = cert;
        this.privateKey = privateKey;
        this.privLevel = privLevel;
    }
    Certificate.prototype.encode = function () {
        return this.cert;
    };
    return Certificate;
}());
exports.Certificate = Certificate;
var ECert = (function (_super) {
    __extends(ECert, _super);
    function ECert(cert, privateKey) {
        _super.call(this, cert, privateKey, PrivacyLevel.Nominal);
        this.cert = cert;
        this.privateKey = privateKey;
    }
    return ECert;
}(Certificate));
exports.ECert = ECert;
var TCert = (function (_super) {
    __extends(TCert, _super);
    function TCert(publicKey, privateKey) {
        _super.call(this, publicKey, privateKey, PrivacyLevel.Anonymous);
        this.publicKey = publicKey;
        this.privateKey = privateKey;
    }
    return TCert;
}(Certificate));
exports.TCert = TCert;
var Chain = (function () {
    function Chain(name) {
        this.peers = [];
        this.securityEnabled = true;
        this.members = {};
        this.tcertBatchSize = 200;
        this.devMode = false;
        this.preFetchMode = true;
        this.name = name;
    }
    Chain.prototype.getName = function () {
        return this.name;
    };
    Chain.prototype.addPeer = function (url, pem) {
        var peer = new Peer(url, this, pem);
        this.peers.push(peer);
        return peer;
    };
    ;
    Chain.prototype.getPeers = function () {
        return this.peers;
    };
    Chain.prototype.getRegistrar = function () {
        return this.registrar;
    };
    Chain.prototype.setRegistrar = function (registrar) {
        this.registrar = registrar;
    };
    Chain.prototype.setMemberServicesUrl = function (url, pem) {
        this.setMemberServices(newMemberServices(url, pem));
    };
    Chain.prototype.getMemberServices = function () {
        return this.memberServices;
    };
    ;
    Chain.prototype.setMemberServices = function (memberServices) {
        this.memberServices = memberServices;
        if (memberServices instanceof MemberServicesImpl) {
            this.cryptoPrimitives = memberServices.getCrypto();
        }
    };
    ;
    Chain.prototype.isSecurityEnabled = function () {
        return this.memberServices !== undefined;
    };
    Chain.prototype.isPreFetchMode = function () {
        return this.preFetchMode;
    };
    Chain.prototype.setPreFetchMode = function (preFetchMode) {
        this.preFetchMode = preFetchMode;
    };
    Chain.prototype.isDevMode = function () {
        return this.devMode;
    };
    Chain.prototype.setDevMode = function (devMode) {
        this.devMode = devMode;
    };
    Chain.prototype.getKeyValStore = function () {
        return this.keyValStore;
    };
    Chain.prototype.setKeyValStore = function (keyValStore) {
        this.keyValStore = keyValStore;
    };
    Chain.prototype.getTCertBatchSize = function () {
        return this.tcertBatchSize;
    };
    Chain.prototype.setTCertBatchSize = function (batchSize) {
        this.tcertBatchSize = batchSize;
    };
    Chain.prototype.getMember = function (name, cb) {
        var self = this;
        cb = cb || nullCB;
        if (!self.keyValStore)
            return cb(Error("No key value store was found.  You must first call Chain.configureKeyValStore or Chain.setKeyValStore"));
        if (!self.memberServices)
            return cb(Error("No member services was found.  You must first call Chain.configureMemberServices or Chain.setMemberServices"));
        self.getMemberHelper(name, function (err, member) {
            if (err)
                return cb(err);
            cb(null, member);
        });
    };
    Chain.prototype.getUser = function (name, cb) {
        return this.getMember(name, cb);
    };
    Chain.prototype.getMemberHelper = function (name, cb) {
        var self = this;
        var member = self.members[name];
        if (member)
            return cb(null, member);
        member = new Member(name, self);
        member.restoreState(function (err) {
            if (err)
                return cb(err);
            cb(null, member);
        });
    };
    Chain.prototype.register = function (registrationRequest, cb) {
        var self = this;
        self.getMember(registrationRequest.enrollmentID, function (err, member) {
            if (err)
                return cb(err);
            member.register(registrationRequest, cb);
        });
    };
    Chain.prototype.enroll = function (name, secret, cb) {
        var self = this;
        self.getMember(name, function (err, member) {
            if (err)
                return cb(err);
            member.enroll(secret, function (err) {
                if (err)
                    return cb(err);
                return cb(null, member);
            });
        });
    };
    Chain.prototype.registerAndEnroll = function (registrationRequest, cb) {
        var self = this;
        self.getMember(registrationRequest.enrollmentID, function (err, member) {
            if (err)
                return cb(err);
            if (member.isEnrolled()) {
                debug("already enrolled");
                return cb(null, member);
            }
            member.registerAndEnroll(registrationRequest, function (err) {
                if (err)
                    return cb(err);
                return cb(null, member);
            });
        });
    };
    Chain.prototype.sendTransaction = function (tx, eventEmitter) {
        var _this = this;
        if (this.peers.length === 0) {
            return eventEmitter.emit('error', new Error(util.format("chain %s has no peers", this.getName())));
        }
        var peers = this.peers;
        var trySendTransaction = function (pidx) {
            if (pidx >= peers.length) {
                eventEmitter.emit('error', "None of " + peers.length + " peers reponding");
                return;
            }
            var p = urlParser.parse(peers[pidx].getUrl());
            var client = new net.Socket();
            var tryNext = function () {
                debug("Skipping unresponsive peer " + peers[pidx].getUrl());
                client.destroy();
                trySendTransaction(pidx + 1);
            };
            client.on('timeout', tryNext);
            client.on('error', tryNext);
            client.connect(p.port, p.hostname, function () {
                if (pidx > 0 && peers === _this.peers)
                    _this.peers = peers.slice(pidx).concat(peers.slice(0, pidx));
                client.destroy();
                peers[pidx].sendTransaction(tx, eventEmitter);
            });
        };
        trySendTransaction(0);
    };
    return Chain;
}());
exports.Chain = Chain;
var Member = (function () {
    function Member(cfg, chain) {
        this.tcerts = [];
        this.arrivalRate = new stats.Rate();
        this.getTCertResponseTime = new stats.ResponseTime();
        this.getTCertWaiters = [];
        this.gettingTCerts = false;
        if (util.isString(cfg)) {
            this.name = cfg;
        }
        else if (util.isObject(cfg)) {
            var req = cfg;
            this.name = req.enrollmentID || req.name;
            this.roles = req.roles || ['fabric.user'];
            this.account = req.account;
            this.affiliation = req.affiliation;
        }
        this.chain = chain;
        this.memberServices = chain.getMemberServices();
        this.keyValStore = chain.getKeyValStore();
        this.keyValStoreName = toKeyValStoreName(this.name);
        this.tcerts = [];
        this.tcertBatchSize = chain.getTCertBatchSize();
    }
    Member.prototype.getName = function () {
        return this.name;
    };
    Member.prototype.getChain = function () {
        return this.chain;
    };
    ;
    Member.prototype.getRoles = function () {
        return this.roles;
    };
    ;
    Member.prototype.setRoles = function (roles) {
        this.roles = roles;
    };
    ;
    Member.prototype.getAccount = function () {
        return this.account;
    };
    ;
    Member.prototype.setAccount = function (account) {
        this.account = account;
    };
    ;
    Member.prototype.getAffiliation = function () {
        return this.affiliation;
    };
    ;
    Member.prototype.setAffiliation = function (affiliation) {
        this.affiliation = affiliation;
    };
    ;
    Member.prototype.getTCertBatchSize = function () {
        if (this.tcertBatchSize === undefined) {
            return this.chain.getTCertBatchSize();
        }
        else {
            return this.tcertBatchSize;
        }
    };
    Member.prototype.setTCertBatchSize = function (batchSize) {
        this.tcertBatchSize = batchSize;
    };
    Member.prototype.getEnrollment = function () {
        return this.enrollment;
    };
    ;
    Member.prototype.isRegistered = function () {
        return this.enrollmentSecret !== undefined;
    };
    Member.prototype.isEnrolled = function () {
        return this.enrollment !== undefined;
    };
    Member.prototype.register = function (registrationRequest, cb) {
        var self = this;
        cb = cb || nullCB;
        if (registrationRequest.enrollmentID !== self.getName()) {
            return cb(Error("registration enrollment ID and member name are not equal"));
        }
        var enrollmentSecret = this.enrollmentSecret;
        if (enrollmentSecret) {
            debug("previously registered, enrollmentSecret=%s", enrollmentSecret);
            return cb(null, enrollmentSecret);
        }
        self.memberServices.register(registrationRequest, self.chain.getRegistrar(), function (err, enrollmentSecret) {
            debug("memberServices.register err=%s, secret=%s", err, enrollmentSecret);
            if (err)
                return cb(err);
            self.enrollmentSecret = enrollmentSecret;
            self.saveState(function (err) {
                if (err)
                    return cb(err);
                cb(null, enrollmentSecret);
            });
        });
    };
    Member.prototype.enroll = function (enrollmentSecret, cb) {
        var self = this;
        cb = cb || nullCB;
        var enrollment = self.enrollment;
        if (enrollment) {
            debug("Previously enrolled, [enrollment=%j]", enrollment);
            return cb(null, enrollment);
        }
        var req = { enrollmentID: self.getName(), enrollmentSecret: enrollmentSecret };
        debug("Enrolling [req=%j]", req);
        self.memberServices.enroll(req, function (err, enrollment) {
            debug("[memberServices.enroll] err=%s, enrollment=%j", err, enrollment);
            if (err)
                return cb(err);
            self.enrollment = enrollment;
            self.enrollment.queryStateKey = self.chain.cryptoPrimitives.generateNonce();
            self.saveState(function (err) {
                if (err)
                    return cb(err);
                debug("[memberServices.enroll] Unmarshalling chainKey");
                var ecdsaChainKey = self.chain.cryptoPrimitives.ecdsaPEMToPublicKey(self.enrollment.chainKey);
                self.enrollment.enrollChainKey = ecdsaChainKey;
                cb(null, enrollment);
            });
        });
    };
    Member.prototype.registerAndEnroll = function (registrationRequest, cb) {
        var self = this;
        cb = cb || nullCB;
        var enrollment = self.enrollment;
        if (enrollment) {
            debug("previously enrolled, enrollment=%j", enrollment);
            return cb(null);
        }
        self.register(registrationRequest, function (err, enrollmentSecret) {
            if (err)
                return cb(err);
            self.enroll(enrollmentSecret, function (err, enrollment) {
                if (err)
                    return cb(err);
                cb(null);
            });
        });
    };
    Member.prototype.deploy = function (deployRequest) {
        console.log("ENTER Member.deploy");
        var tx = this.newTransactionContext();
        tx.deploy(deployRequest);
        return tx;
    };
    Member.prototype.invoke = function (invokeRequest) {
        var tx = this.newTransactionContext();
        tx.invoke(invokeRequest);
        return tx;
    };
    Member.prototype.query = function (queryRequest) {
        var tx = this.newTransactionContext();
        tx.query(queryRequest);
        return tx;
    };
    Member.prototype.newTransactionContext = function (tcert) {
        return new TransactionContext(this, tcert);
    };
    Member.prototype.getUserCert = function (cb) {
        this.getNextTCert(cb);
    };
    Member.prototype.getNextTCert = function (cb) {
        var self = this;
        if (!self.isEnrolled()) {
            return cb(Error(util.format("user '%s' is not enrolled", self.getName())));
        }
        self.arrivalRate.tick();
        var tcert = self.tcerts.length > 0 ? self.tcerts.shift() : undefined;
        if (tcert) {
            return cb(null, tcert);
        }
        else {
            self.getTCertWaiters.push(cb);
        }
        if (self.shouldGetTCerts()) {
            self.getTCerts();
        }
    };
    Member.prototype.shouldGetTCerts = function () {
        var self = this;
        if (self.gettingTCerts) {
            debug("shouldGetTCerts: no, already getting tcerts");
            return false;
        }
        if (self.tcerts.length == 0) {
            debug("shouldGetTCerts: yes, we have no tcerts");
            return true;
        }
        if (!self.chain.isPreFetchMode()) {
            debug("shouldGetTCerts: no, prefetch disabled");
            return false;
        }
        var arrivalRate = self.arrivalRate.getValue();
        var responseTime = self.getTCertResponseTime.getValue() + 1000;
        var tcertThreshold = arrivalRate * responseTime;
        var tcertCount = self.tcerts.length;
        var result = tcertCount <= tcertThreshold;
        debug(util.format("shouldGetTCerts: %s, threshold=%s, count=%s, rate=%s, responseTime=%s", result, tcertThreshold, tcertCount, arrivalRate, responseTime));
        return result;
    };
    Member.prototype.getTCerts = function () {
        var self = this;
        var req = {
            name: self.getName(),
            enrollment: self.enrollment,
            num: self.getTCertBatchSize()
        };
        self.getTCertResponseTime.start();
        self.memberServices.getTCertBatch(req, function (err, tcerts) {
            if (err) {
                self.getTCertResponseTime.cancel();
                while (self.getTCertWaiters.length > 0) {
                    self.getTCertWaiters.shift()(err);
                }
                return;
            }
            self.getTCertResponseTime.stop();
            while (tcerts.length > 0) {
                self.tcerts.push(tcerts.shift());
            }
            while (self.getTCertWaiters.length > 0 && self.tcerts.length > 0) {
                self.getTCertWaiters.shift()(null, self.tcerts.shift());
            }
        });
    };
    Member.prototype.saveState = function (cb) {
        var self = this;
        self.keyValStore.setValue(self.keyValStoreName, self.toString(), cb);
    };
    Member.prototype.restoreState = function (cb) {
        var self = this;
        self.keyValStore.getValue(self.keyValStoreName, function (err, memberStr) {
            if (err)
                return cb(err);
            if (memberStr) {
                self.fromString(memberStr);
            }
            cb(null);
        });
    };
    Member.prototype.fromString = function (str) {
        var state = JSON.parse(str);
        if (state.name !== this.getName())
            throw Error("name mismatch: '" + state.name + "' does not equal '" + this.getName() + "'");
        this.name = state.name;
        this.roles = state.roles;
        this.account = state.account;
        this.affiliation = state.affiliation;
        this.enrollmentSecret = state.enrollmentSecret;
        this.enrollment = state.enrollment;
    };
    Member.prototype.toString = function () {
        var self = this;
        var state = {
            name: self.name,
            roles: self.roles,
            account: self.account,
            affiliation: self.affiliation,
            enrollmentSecret: self.enrollmentSecret,
            enrollment: self.enrollment
        };
        return JSON.stringify(state);
    };
    return Member;
}());
exports.Member = Member;
var TransactionContext = (function (_super) {
    __extends(TransactionContext, _super);
    function TransactionContext(member, tcert) {
        _super.call(this);
        this.member = member;
        this.chain = member.getChain();
        this.memberServices = this.chain.getMemberServices();
        this.tcert = tcert;
        this.nonce = this.chain.cryptoPrimitives.generateNonce();
    }
    TransactionContext.prototype.getMember = function () {
        return this.member;
    };
    TransactionContext.prototype.getChain = function () {
        return this.chain;
    };
    ;
    TransactionContext.prototype.getMemberServices = function () {
        return this.memberServices;
    };
    ;
    TransactionContext.prototype.deploy = function (deployRequest) {
        console.log("TransactionContext.deploy");
        console.log("Received deploy request: %j", deployRequest);
        var self = this;
        self.getMyTCert(function (err) {
            if (err) {
                console.log('Failed getting a new TCert [%s]', err);
                self.emit('error', err);
                return self;
            }
            console.log("Got a TCert successfully, continue...");
            self.newBuildOrDeployTransaction(deployRequest, false, function (err, deployTx) {
                if (err) {
                    console.log("Error in newBuildOrDeployTransaction [%s]", err);
                    self.emit('error', err);
                    return self;
                }
                console.log("Calling TransactionContext.execute");
                return self.execute(deployTx);
            });
        });
        return self;
    };
    TransactionContext.prototype.invoke = function (invokeRequest) {
        var self = this;
        self.getMyTCert(function (err, tcert) {
            if (err) {
                debug('Failed getting a new TCert [%s]', err);
                self.emit('error', err);
                return self;
            }
            return self.execute(self.newInvokeOrQueryTransaction(invokeRequest, true));
        });
        return self;
    };
    TransactionContext.prototype.query = function (queryRequest) {
        var self = this;
        self.getMyTCert(function (err, tcert) {
            if (err) {
                debug('Failed getting a new TCert [%s]', err);
                self.emit('error', err);
                return self;
            }
            return self.execute(self.newInvokeOrQueryTransaction(queryRequest, false));
        });
        return self;
    };
    TransactionContext.prototype.execute = function (tx) {
        debug('Executing transaction [%j]', tx);
        var self = this;
        self.getMyTCert(function (err, tcert) {
            if (err) {
                debug('Failed getting a new TCert [%s]', err);
                return self.emit('error', err);
            }
            if (tcert) {
                tx.setNonce(self.nonce);
                debug('Process Confidentiality...');
                self.processConfidentiality(tx);
                debug('Sign transaction...');
                tx.setCert(tcert.publicKey);
                var txBytes = tx.toBuffer();
                var derSignature = self.chain.cryptoPrimitives.ecdsaSign(tcert.privateKey.getPrivate('hex'), txBytes).toDER();
                tx.setSignature(new Buffer(derSignature));
                debug('Send transaction...');
                debug('Confidentiality: ', tx.getConfidentialityLevel());
                if (tx.getConfidentialityLevel() == _fabricProto.ConfidentialityLevel.CONFIDENTIAL &&
                    tx.getType() == _fabricProto.Transaction.Type.CHAINCODE_QUERY) {
                    var emitter = new events.EventEmitter();
                    emitter.on("complete", function (results) {
                        debug("Encrypted: [%j]", results);
                        var res = self.decryptResult(results);
                        debug("Decrypted: [%j]", res);
                        self.emit("complete", res);
                    });
                    emitter.on("error", function (results) {
                        self.emit("error", results);
                    });
                    self.getChain().sendTransaction(tx, emitter);
                }
                else {
                    self.getChain().sendTransaction(tx, self);
                }
            }
            else {
                debug('Missing TCert...');
                return self.emit('error', 'Missing TCert.');
            }
        });
        return self;
    };
    TransactionContext.prototype.getMyTCert = function (cb) {
        var self = this;
        if (!self.getChain().isSecurityEnabled() || self.tcert) {
            debug('[TransactionContext] TCert already cached.');
            return cb(null, self.tcert);
        }
        debug('[TransactionContext] No TCert cached. Retrieving one.');
        this.member.getNextTCert(function (err, tcert) {
            if (err)
                return cb(err);
            self.tcert = tcert;
            return cb(null, tcert);
        });
    };
    TransactionContext.prototype.processConfidentiality = function (transaction) {
        if (transaction.getConfidentialityLevel() != _fabricProto.ConfidentialityLevel.CONFIDENTIAL) {
            return;
        }
        debug('Process Confidentiality ...');
        var self = this;
        transaction.setConfidentialityProtocolVersion('1.2');
        var txKey = self.chain.cryptoPrimitives.eciesKeyGen();
        debug('txkey [%j]', txKey.pubKeyObj.pubKeyHex);
        debug('txKey.prvKeyObj %j', txKey.prvKeyObj.toString());
        var privBytes = self.chain.cryptoPrimitives.ecdsaPrivateKeyToASN1(txKey.prvKeyObj.prvKeyHex);
        debug('privBytes %s', privBytes.toString());
        var stateKey;
        if (transaction.getType() == _fabricProto.Transaction.Type.CHAINCODE_DEPLOY) {
            stateKey = new Buffer(self.chain.cryptoPrimitives.aesKeyGen());
        }
        else if (transaction.getType() == _fabricProto.Transaction.Type.CHAINCODE_INVOKE) {
            stateKey = new Buffer([]);
        }
        else {
            debug('Generate state key...');
            stateKey = new Buffer(self.chain.cryptoPrimitives.hmacAESTruncated(self.member.getEnrollment().queryStateKey, [CONFIDENTIALITY_1_2_STATE_KD_C6].concat(self.nonce)));
        }
        var chainCodeValidatorMessage1_2 = new asn1Builder.Ber.Writer();
        chainCodeValidatorMessage1_2.startSequence();
        chainCodeValidatorMessage1_2.writeBuffer(privBytes, 4);
        if (stateKey.length != 0) {
            debug('STATE KEY %j', stateKey);
            chainCodeValidatorMessage1_2.writeBuffer(stateKey, 4);
        }
        else {
            chainCodeValidatorMessage1_2.writeByte(4);
            chainCodeValidatorMessage1_2.writeLength(0);
        }
        chainCodeValidatorMessage1_2.endSequence();
        debug(chainCodeValidatorMessage1_2.buffer);
        debug('Using chain key [%j]', self.member.getEnrollment().chainKey);
        var ecdsaChainKey = self.chain.cryptoPrimitives.ecdsaPEMToPublicKey(self.member.getEnrollment().chainKey);
        var encMsgToValidators = self.chain.cryptoPrimitives.eciesEncryptECDSA(ecdsaChainKey, chainCodeValidatorMessage1_2.buffer);
        transaction.setToValidators(encMsgToValidators);
        var encryptedChaincodeID = self.chain.cryptoPrimitives.eciesEncrypt(txKey.pubKeyObj, transaction.getChaincodeID().buffer);
        transaction.setChaincodeID(encryptedChaincodeID);
        var encryptedPayload = self.chain.cryptoPrimitives.eciesEncrypt(txKey.pubKeyObj, transaction.getPayload().buffer);
        transaction.setPayload(encryptedPayload);
        if (transaction.getMetadata() != null && transaction.getMetadata().buffer != null) {
            debug('Metadata [%j]', transaction.getMetadata().buffer);
            var encryptedMetadata = self.chain.cryptoPrimitives.eciesEncrypt(txKey.pubKeyObj, transaction.getMetadata().buffer);
            transaction.setMetadata(encryptedMetadata);
        }
    };
    TransactionContext.prototype.decryptResult = function (ct) {
        var key = new Buffer(this.chain.cryptoPrimitives.hmacAESTruncated(this.member.getEnrollment().queryStateKey, [CONFIDENTIALITY_1_2_STATE_KD_C6].concat(this.nonce)));
        debug('Decrypt Result [%s]', ct.toString('hex'));
        return this.chain.cryptoPrimitives.aes256GCMDecrypt(key, ct);
    };
    TransactionContext.prototype.newBuildOrDeployTransaction = function (request, isBuildRequest, cb) {
        console.log("newBuildOrDeployTransaction");
        var self = this;
        var goPath = process.env.GOPATH;
        console.log("$GOPATH: " + goPath);
        var projDir = goPath + "/src/" + request.chaincodePath;
        console.log("projDir: " + projDir);
        var hash = sdk_util.GenerateParameterHash(request.chaincodePath, request.fcn, request.args);
        hash = sdk_util.GenerateDirectoryHash(goPath + "/src/", request.chaincodePath, hash);
        console.log("hash: " + hash);
        var dockerFileContents = "from hyperledger/fabric-baseimage" + "\n" +
            "COPY . $GOPATH/src/build-chaincode/" + "\n" +
            "WORKDIR $GOPATH" + "\n\n" +
            "RUN go install build-chaincode && cp src/build-chaincode/vendor/github.com/hyperledger/fabric/peer/core.yaml $GOPATH/bin && mv $GOPATH/bin/build-chaincode $GOPATH/bin/%s";
        dockerFileContents = util.format(dockerFileContents, hash);
        var dockerFilePath = projDir + "/Dockerfile";
        fs.writeFile(dockerFilePath, dockerFileContents, function (err) {
            if (err) {
                console.log(util.format("Error writing file [%s]: %s", dockerFilePath, err));
                return cb(Error(util.format("Error writing file [%s]: %s", dockerFilePath, err)));
            }
            console.log("Created Dockerfile at [%s]", dockerFilePath);
            var targzFilePath = "/tmp/deployment-package.tar.gz";
            var tarball = new targz({}, {
                fromBase: true
            });
            tarball.compress(projDir, targzFilePath, function (err) {
                if (err) {
                    console.log(util.format("Error creating deployment archive [%s]: %s", targzFilePath, err));
                    return cb(Error(util.format("Error creating deployment archive [%s]: %s", targzFilePath, err)));
                }
                console.log(util.format("Created deployment archive at [%s]", targzFilePath));
                var tx = new _fabricProto.Transaction();
                if (isBuildRequest) {
                    tx.setType(_fabricProto.Transaction.Type.CHAINCODE_BUILD);
                }
                else {
                    tx.setType(_fabricProto.Transaction.Type.CHAINCODE_DEPLOY);
                }
                var chaincodeID = new _chaincodeProto.ChaincodeID();
                chaincodeID.setName(hash);
                console.log("chaincodeID: " + JSON.stringify(chaincodeID));
                tx.setChaincodeID(chaincodeID.toBuffer());
                var chaincodeSpec = new _chaincodeProto.ChaincodeSpec();
                chaincodeSpec.setType(_chaincodeProto.ChaincodeSpec.Type.GOLANG);
                chaincodeSpec.setChaincodeID(chaincodeID);
                var chaincodeInput = new _chaincodeProto.ChaincodeInput();
                chaincodeInput.setFunction(request.fcn);
                chaincodeInput.setArgs(request.args);
                chaincodeSpec.setCtorMsg(chaincodeInput);
                console.log("chaincodeSpec: " + JSON.stringify(chaincodeSpec));
                var chaincodeDeploymentSpec = new _chaincodeProto.ChaincodeDeploymentSpec();
                chaincodeDeploymentSpec.setChaincodeSpec(chaincodeSpec);
                fs.readFile(targzFilePath, function (err, data) {
                    if (err) {
                        console.log(util.format("Error reading deployment archive [%s]: %s", targzFilePath, err));
                        return cb(Error(util.format("Error reading deployment archive [%s]: %s", targzFilePath, err)));
                    }
                    console.log(util.format("Read in deployment archive from [%s]", targzFilePath));
                    chaincodeDeploymentSpec.setCodePackage(data);
                    tx.setPayload(chaincodeDeploymentSpec.toBuffer());
                    if (self.chain.isDevMode()) {
                        tx.setUuid(request.chaincodeID);
                    }
                    else {
                        tx.setUuid(sdk_util.GenerateUUID());
                    }
                    tx.setTimestamp(sdk_util.GenerateTimestamp());
                    if (request.confidential) {
                        debug("Set confidentiality level to CONFIDENTIAL");
                        tx.setConfidentialityLevel(_fabricProto.ConfidentialityLevel.CONFIDENTIAL);
                    }
                    else {
                        debug("Set confidentiality level to PUBLIC");
                        tx.setConfidentialityLevel(_fabricProto.ConfidentialityLevel.PUBLIC);
                    }
                    if (request.metadata) {
                        tx.setMetadata(request.metadata);
                    }
                    if (request.userCert) {
                        var certRaw = new Buffer(self.tcert.publicKey);
                        var nonceRaw = new Buffer(self.nonce);
                        var bindingMsg = Buffer.concat([certRaw, nonceRaw]);
                        this.binding = new Buffer(self.chain.cryptoPrimitives.hash(bindingMsg), 'hex');
                        var ctor = chaincodeSpec.getCtorMsg().toBuffer();
                        var txmsg = Buffer.concat([ctor, this.binding]);
                        var mdsig = self.chain.cryptoPrimitives.ecdsaSign(request.userCert.privateKey.getPrivate('hex'), txmsg);
                        var sigma = new Buffer(mdsig.toDER());
                        tx.setMetadata(sigma);
                    }
                    fs.unlink(targzFilePath, function (err) {
                        if (err) {
                            console.log(util.format("Error deleting temporary archive [%s]: %s", targzFilePath, err));
                            return cb(Error(util.format("Error deleting temporary archive [%s]: %s", targzFilePath, err)));
                        }
                        console.log("Temporary archive deleted successfully ---> " + targzFilePath);
                        fs.unlink(dockerFilePath, function (err) {
                            if (err) {
                                console.log(util.format("Error deleting temporary file [%s]: %s", dockerFilePath, err));
                                return cb(Error(util.format("Error deleting temporary file [%s]: %s", dockerFilePath, err)));
                            }
                            console.log("File deleted successfully ---> " + dockerFilePath);
                            return cb(null, tx);
                        });
                    });
                });
            });
        });
    };
    TransactionContext.prototype.newInvokeOrQueryTransaction = function (request, isInvokeRequest) {
        var self = this;
        var tx = new _fabricProto.Transaction();
        if (isInvokeRequest) {
            tx.setType(_fabricProto.Transaction.Type.CHAINCODE_INVOKE);
        }
        else {
            tx.setType(_fabricProto.Transaction.Type.CHAINCODE_QUERY);
        }
        var chaincodeID = new _chaincodeProto.ChaincodeID();
        chaincodeID.setName(request.chaincodeID);
        debug("newInvokeOrQueryTransaction: request=%j, chaincodeID=%s", request, JSON.stringify(chaincodeID));
        tx.setChaincodeID(chaincodeID.toBuffer());
        var chaincodeSpec = new _chaincodeProto.ChaincodeSpec();
        chaincodeSpec.setType(_chaincodeProto.ChaincodeSpec.Type.GOLANG);
        chaincodeSpec.setChaincodeID(chaincodeID);
        var chaincodeInput = new _chaincodeProto.ChaincodeInput();
        chaincodeInput.setFunction(request.fcn);
        chaincodeInput.setArgs(request.args);
        chaincodeSpec.setCtorMsg(chaincodeInput);
        var chaincodeInvocationSpec = new _chaincodeProto.ChaincodeInvocationSpec();
        chaincodeInvocationSpec.setChaincodeSpec(chaincodeSpec);
        tx.setPayload(chaincodeInvocationSpec.toBuffer());
        tx.setUuid(generateUUID());
        tx.setTimestamp(generateTimestamp());
        if (request.confidential) {
            debug('Set confidentiality on');
            tx.setConfidentialityLevel(_fabricProto.ConfidentialityLevel.CONFIDENTIAL);
        }
        else {
            debug('Set confidentiality on');
            tx.setConfidentialityLevel(_fabricProto.ConfidentialityLevel.PUBLIC);
        }
        if (request.metadata) {
            tx.setMetadata(request.metadata);
        }
        if (request.userCert) {
            var certRaw = new Buffer(self.tcert.publicKey);
            var nonceRaw = new Buffer(self.nonce);
            var bindingMsg = Buffer.concat([certRaw, nonceRaw]);
            this.binding = new Buffer(self.chain.cryptoPrimitives.hash(bindingMsg), 'hex');
            var ctor = chaincodeSpec.getCtorMsg().toBuffer();
            var txmsg = Buffer.concat([ctor, this.binding]);
            var mdsig = self.chain.cryptoPrimitives.ecdsaSign(request.userCert.privateKey.getPrivate('hex'), txmsg);
            var sigma = new Buffer(mdsig.toDER());
            tx.setMetadata(sigma);
        }
        return tx;
    };
    return TransactionContext;
}(events.EventEmitter));
exports.TransactionContext = TransactionContext;
var Peer = (function () {
    function Peer(url, chain, pem) {
        this.sendTransaction = function (tx, eventEmitter) {
            var self = this;
            debug("peer.sendTransaction: sending %j", tx);
            self.peerClient.processTransaction(tx, function (err, response) {
                if (err) {
                    debug("peer.sendTransaction: error=%j", err);
                    return eventEmitter.emit('error', err);
                }
                console.log("peer.sendTransaction: received %j", response);
                var txType = tx.getType();
                switch (txType) {
                    case _fabricProto.Transaction.Type.CHAINCODE_DEPLOY:
                        if (response.status === "SUCCESS") {
                            if (!response.msg || response.msg === "") {
                                eventEmitter.emit('error', 'the deploy response is missing the transaction UUID');
                            }
                            else {
                                eventEmitter.emit('complete', response.msg);
                            }
                        }
                        else {
                            eventEmitter.emit('error', response.msg);
                        }
                        break;
                    case _fabricProto.Transaction.Type.CHAINCODE_INVOKE:
                        if (response.status === "SUCCESS") {
                            if (!response.msg || response.msg === "") {
                                eventEmitter.emit('error', 'the invoke response is missing the transaction UUID');
                            }
                            else {
                                eventEmitter.emit('submitted', response.msg);
                                self.waitToComplete(eventEmitter);
                            }
                        }
                        else {
                            eventEmitter.emit('error', response.msg);
                        }
                        break;
                    case _fabricProto.Transaction.Type.CHAINCODE_QUERY:
                        if (response.status === "SUCCESS") {
                            eventEmitter.emit('complete', response.msg);
                        }
                        else {
                            eventEmitter.emit('error', response.msg);
                        }
                        break;
                    default:
                        eventEmitter.emit('error', new Error("processTransaction for this transaction type is not yet implemented!"));
                }
            });
        };
        this.url = url;
        this.chain = chain;
        this.ep = new Endpoint(url, pem);
        this.peerClient = new _fabricProto.Peer(this.ep.addr, this.ep.creds);
    }
    Peer.prototype.getChain = function () {
        return this.chain;
    };
    Peer.prototype.getUrl = function () {
        return this.url;
    };
    Peer.prototype.waitToComplete = function (eventEmitter) {
        debug("waiting 5 seconds before emitting complete event");
        var emitComplete = function () {
            debug("emitting completion event");
            eventEmitter.emit('complete');
        };
        setTimeout(emitComplete, 5000);
    };
    Peer.prototype.remove = function () {
        throw Error("TODO: implement");
    };
    return Peer;
}());
exports.Peer = Peer;
var Endpoint = (function () {
    function Endpoint(url, pem) {
        var purl = parseUrl(url);
        var protocol = purl.protocol.toLowerCase();
        if (protocol === 'grpc') {
            this.addr = purl.host;
            this.creds = grpc.credentials.createInsecure();
        }
        else if (protocol === 'grpcs') {
            this.addr = purl.host;
            this.creds = grpc.credentials.createSsl(new Buffer(pem));
        }
        else {
            throw Error("invalid protocol: " + protocol);
        }
    }
    return Endpoint;
}());
var MemberServicesImpl = (function () {
    function MemberServicesImpl(url, pem) {
        var ep = new Endpoint(url, pem);
        var options = {
            'grpc.ssl_target_name_override': 'tlsca',
            'grpc.default_authority': 'tlsca'
        };
        this.ecaaClient = new _caProto.ECAA(ep.addr, ep.creds, options);
        this.ecapClient = new _caProto.ECAP(ep.addr, ep.creds, options);
        this.tcapClient = new _caProto.TCAP(ep.addr, ep.creds, options);
        this.tlscapClient = new _caProto.TLSCAP(ep.addr, ep.creds, options);
        this.cryptoPrimitives = new crypto.Crypto(DEFAULT_HASH_ALGORITHM, DEFAULT_SECURITY_LEVEL);
    }
    MemberServicesImpl.prototype.getSecurityLevel = function () {
        return this.cryptoPrimitives.getSecurityLevel();
    };
    MemberServicesImpl.prototype.setSecurityLevel = function (securityLevel) {
        this.cryptoPrimitives.setSecurityLevel(securityLevel);
    };
    MemberServicesImpl.prototype.getHashAlgorithm = function () {
        return this.cryptoPrimitives.getHashAlgorithm();
    };
    MemberServicesImpl.prototype.setHashAlgorithm = function (hashAlgorithm) {
        this.cryptoPrimitives.setHashAlgorithm(hashAlgorithm);
    };
    MemberServicesImpl.prototype.getCrypto = function () {
        return this.cryptoPrimitives;
    };
    MemberServicesImpl.prototype.register = function (req, registrar, cb) {
        var self = this;
        debug("MemberServicesImpl.register: req=%j", req);
        if (!req.enrollmentID)
            return cb(new Error("missing req.enrollmentID"));
        if (!registrar)
            return cb(new Error("chain registrar is not set"));
        var protoReq = new _caProto.RegisterUserReq();
        protoReq.setId({ id: req.enrollmentID });
        protoReq.setRole(rolesToMask(req.roles));
        protoReq.setAccount(req.account);
        protoReq.setAffiliation(req.affiliation);
        var protoRegistrar = new _caProto.Registrar();
        protoRegistrar.setId({ id: registrar.getName() });
        if (req.registrar) {
            if (req.registrar.roles) {
                protoRegistrar.setRoles(req.registrar.roles);
            }
            if (req.registrar.delegateRoles) {
                protoRegistrar.setDelegateRoles(req.registrar.delegateRoles);
            }
        }
        protoReq.setRegistrar(protoRegistrar);
        var buf = protoReq.toBuffer();
        var signKey = self.cryptoPrimitives.ecdsaKeyFromPrivate(registrar.getEnrollment().key, 'hex');
        var sig = self.cryptoPrimitives.ecdsaSign(signKey, buf);
        protoReq.setSig(new _caProto.Signature({
            type: _caProto.CryptoType.ECDSA,
            r: new Buffer(sig.r.toString()),
            s: new Buffer(sig.s.toString())
        }));
        self.ecaaClient.registerUser(protoReq, function (err, token) {
            debug("register %j: err=%j, token=%s", protoReq, err, token);
            if (cb)
                return cb(err, token ? token.tok.toString() : null);
        });
    };
    MemberServicesImpl.prototype.enroll = function (req, cb) {
        var self = this;
        cb = cb || nullCB;
        debug("[MemberServicesImpl.enroll] [%j]", req);
        if (!req.enrollmentID)
            return cb(Error("req.enrollmentID is not set"));
        if (!req.enrollmentSecret)
            return cb(Error("req.enrollmentSecret is not set"));
        debug("[MemberServicesImpl.enroll] Generating keys...");
        var signingKeyPair = self.cryptoPrimitives.ecdsaKeyGen();
        var spki = new asn1.x509.SubjectPublicKeyInfo(signingKeyPair.pubKeyObj);
        var encryptionKeyPair = self.cryptoPrimitives.ecdsaKeyGen();
        var spki2 = new asn1.x509.SubjectPublicKeyInfo(encryptionKeyPair.pubKeyObj);
        debug("[MemberServicesImpl.enroll] Generating keys...done!");
        var eCertCreateRequest = new _caProto.ECertCreateReq();
        var timestamp = new _timeStampProto({ seconds: Date.now() / 1000, nanos: 0 });
        eCertCreateRequest.setTs(timestamp);
        eCertCreateRequest.setId({ id: req.enrollmentID });
        eCertCreateRequest.setTok({ tok: new Buffer(req.enrollmentSecret) });
        debug("[MemberServicesImpl.enroll] Generating request! %j", spki.getASN1Object().getEncodedHex());
        var signPubKey = new _caProto.PublicKey({
            type: _caProto.CryptoType.ECDSA,
            key: new Buffer(spki.getASN1Object().getEncodedHex(), 'hex')
        });
        eCertCreateRequest.setSign(signPubKey);
        debug("[MemberServicesImpl.enroll] Adding signing key!");
        var encPubKey = new _caProto.PublicKey({
            type: _caProto.CryptoType.ECDSA,
            key: new Buffer(spki2.getASN1Object().getEncodedHex(), 'hex')
        });
        eCertCreateRequest.setEnc(encPubKey);
        debug("[MemberServicesImpl.enroll] Assding encryption key!");
        debug("[MemberServicesImpl.enroll] [Contact ECA] %j ", eCertCreateRequest);
        self.ecapClient.createCertificatePair(eCertCreateRequest, function (err, eCertCreateResp) {
            if (err) {
                debug("[MemberServicesImpl.enroll] failed to create cert pair: err=%j", err);
                return cb(err);
            }
            var cipherText = eCertCreateResp.tok.tok;
            var decryptedTokBytes = self.cryptoPrimitives.eciesDecrypt(encryptionKeyPair.prvKeyObj, cipherText);
            eCertCreateRequest.setTok({ tok: decryptedTokBytes });
            eCertCreateRequest.setSig(null);
            var buf = eCertCreateRequest.toBuffer();
            var signKey = self.cryptoPrimitives.ecdsaKeyFromPrivate(signingKeyPair.prvKeyObj.prvKeyHex, 'hex');
            var sig = self.cryptoPrimitives.ecdsaSign(signKey, buf);
            eCertCreateRequest.setSig(new _caProto.Signature({
                type: _caProto.CryptoType.ECDSA,
                r: new Buffer(sig.r.toString()),
                s: new Buffer(sig.s.toString())
            }));
            self.ecapClient.createCertificatePair(eCertCreateRequest, function (err, eCertCreateResp) {
                if (err)
                    return cb(err);
                debug('[MemberServicesImpl.enroll] eCertCreateResp : [%j]' + eCertCreateResp);
                var enrollment = {
                    key: signingKeyPair.prvKeyObj.prvKeyHex,
                    cert: eCertCreateResp.certs.sign.toString('hex'),
                    chainKey: eCertCreateResp.pkchain.toString('hex')
                };
                cb(null, enrollment);
            });
        });
    };
    MemberServicesImpl.prototype.getTCertBatch = function (req, cb) {
        var self = this;
        cb = cb || nullCB;
        var timestamp = new _timeStampProto({ seconds: Date.now() / 1000, nanos: 0 });
        var tCertCreateSetReq = new _caProto.TCertCreateSetReq();
        tCertCreateSetReq.setTs(timestamp);
        tCertCreateSetReq.setId({ id: req.name });
        tCertCreateSetReq.setNum(req.num);
        var buf = tCertCreateSetReq.toBuffer();
        var signKey = self.cryptoPrimitives.ecdsaKeyFromPrivate(req.enrollment.key, 'hex');
        var sig = self.cryptoPrimitives.ecdsaSign(signKey, buf);
        tCertCreateSetReq.setSig(new _caProto.Signature({
            type: _caProto.CryptoType.ECDSA,
            r: new Buffer(sig.r.toString()),
            s: new Buffer(sig.s.toString())
        }));
        self.tcapClient.createCertificateSet(tCertCreateSetReq, function (err, resp) {
            if (err)
                return cb(err);
            cb(null, self.processTCertBatch(req, resp));
        });
    };
    MemberServicesImpl.prototype.processTCertBatch = function (req, resp) {
        var self = this;
        var enrollKey = req.enrollment.key;
        var tCertOwnerKDFKey = resp.certs.key;
        var tCerts = resp.certs.certs;
        var byte1 = new Buffer(1);
        byte1.writeUInt8(0x1, 0);
        var byte2 = new Buffer(1);
        byte2.writeUInt8(0x2, 0);
        var tCertOwnerEncryptKey = self.cryptoPrimitives.hmac(tCertOwnerKDFKey, byte1).slice(0, 32);
        var expansionKey = self.cryptoPrimitives.hmac(tCertOwnerKDFKey, byte2);
        var tCertBatch = [];
        for (var i = 0; i < tCerts.length; i++) {
            var tCert = tCerts[i];
            var x509Certificate = void 0;
            try {
                x509Certificate = new crypto.X509Certificate(tCert.cert);
            }
            catch (ex) {
                debug('Warning: problem parsing certificate bytes; retrying ... ', ex);
                continue;
            }
            var tCertIndexCT = x509Certificate.criticalExtension(crypto.TCertEncTCertIndex);
            var tCertIndex = self.cryptoPrimitives.aesCBCPKCS7Decrypt(tCertOwnerEncryptKey, tCertIndexCT);
            var expansionValue = self.cryptoPrimitives.hmac(expansionKey, tCertIndex);
            var one = new BN(1);
            var k = new BN(expansionValue);
            var n = self.cryptoPrimitives.ecdsaKeyFromPrivate(enrollKey, 'hex').ec.curve.n.sub(one);
            k = k.mod(n).add(one);
            var D = self.cryptoPrimitives.ecdsaKeyFromPrivate(enrollKey, 'hex').getPrivate().add(k);
            var pubHex = self.cryptoPrimitives.ecdsaKeyFromPrivate(enrollKey, 'hex').getPublic('hex');
            D = D.mod(self.cryptoPrimitives.ecdsaKeyFromPublic(pubHex, 'hex').ec.curve.n);
            var tcert = new TCert(tCert.cert, self.cryptoPrimitives.ecdsaKeyFromPrivate(D, 'hex'));
            tCertBatch.push(tcert);
        }
        if (tCertBatch.length == 0) {
            throw Error('Failed fetching TCertBatch. No valid TCert received.');
        }
        return tCertBatch;
    };
    return MemberServicesImpl;
}());
function newMemberServices(url, pem) {
    return new MemberServicesImpl(url, pem);
}
var FileKeyValStore = (function () {
    function FileKeyValStore(dir) {
        this.setValue = function (name, value, cb) {
            var path = this.dir + '/' + name;
            fs.writeFile(path, value, cb);
        };
        this.dir = dir;
        if (!fs.existsSync(dir)) {
            fs.mkdirSync(dir);
        }
    }
    FileKeyValStore.prototype.getValue = function (name, cb) {
        var path = this.dir + '/' + name;
        fs.readFile(path, 'utf8', function (err, data) {
            if (err) {
                if (err.code !== 'ENOENT')
                    return cb(err);
                return cb(null, null);
            }
            return cb(null, data);
        });
    };
    return FileKeyValStore;
}());
function generateUUID() {
    return uuid.v4();
}
;
function generateTimestamp() {
    return new _timeStampProto({ seconds: Date.now() / 1000, nanos: 0 });
}
function toKeyValStoreName(name) {
    return "member." + name;
}
function bluemixInit() {
    var vcap = process.env['VCAP_SERVICES'];
    if (!vcap)
        return false;
    return true;
}
function nullCB() {
}
function isString(obj) {
    return (typeof obj === 'string' || obj instanceof String);
}
function isObject(obj) {
    return (!!obj) && (obj.constructor === Object);
}
function isFunction(fcn) {
    return (typeof fcn === 'function');
}
function parseUrl(url) {
    var purl = urlParser.parse(url, true);
    var protocol = purl.protocol;
    if (endsWith(protocol, ":")) {
        purl.protocol = protocol.slice(0, -1);
    }
    return purl;
}
function rolesToMask(roles) {
    var mask = 0;
    if (roles) {
        for (var role in roles) {
            switch (role) {
                case 'client':
                    mask |= 1;
                    break;
                case 'peer':
                    mask |= 2;
                    break;
                case 'validator':
                    mask |= 4;
                    break;
                case 'auditor':
                    mask |= 8;
                    break;
            }
        }
    }
    if (mask === 0)
        mask = 1;
    return mask;
}
function endsWith(str, suffix) {
    return str.length >= suffix.length && str.substr(str.length - suffix.length) === suffix;
}
;
function newChain(name) {
    var chain = _chains[name];
    if (chain)
        throw Error(util.format("chain %s already exists", name));
    chain = new Chain(name);
    _chains[name] = chain;
    return chain;
}
exports.newChain = newChain;
function getChain(chainName, create) {
    var chain = _chains[chainName];
    if (!chain && create) {
        chain = newChain(name);
    }
    return chain;
}
exports.getChain = getChain;
function newFileKeyValStore(dir) {
    return new FileKeyValStore(dir);
}
exports.newFileKeyValStore = newFileKeyValStore;
//# sourceMappingURL=hlc.js.map