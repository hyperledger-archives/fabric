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
"use strict";
var __extends = (this && this.__extends) || function (d, b) {
    for (var p in b) if (b.hasOwnProperty(p)) d[p] = b[p];
    function __() { this.constructor = d; }
    d.prototype = b === null ? Object.create(b) : (__.prototype = b.prototype, new __());
};
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
 *          the server side transaction processing path.
 */
// Instruct boringssl to use ECC for tls.
process.env['GRPC_SSL_CIPHER_SUITES'] = 'HIGH+ECDSA';
var debugModule = require('debug');
var fs = require('fs');
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
var events = require('events');
var debug = debugModule('hlc'); // 'hlc' stands for 'HyperLedger Client'
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
// The base Certificate class
var Certificate = (function () {
    function Certificate(cert, privateKey, 
        /** Denoting if the Certificate is anonymous or carrying its owner's identity. */
        privLevel) {
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
/**
 * Enrollment certificate.
 */
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
/**
 * Transaction certificate.
 */
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
/**
 * The class representing a chain with which the client SDK interacts.
 */
var Chain = (function () {
    function Chain(name) {
        // The peers on this chain to which the client can connect
        this.peers = [];
        // Security enabled flag
        this.securityEnabled = true;
        // A member cache associated with this chain
        // TODO: Make an LRU to limit size of member cache
        this.members = {};
        // The number of tcerts to get in each batch
        this.tcertBatchSize = 200;
        // Is in dev mode or network mode
        this.devMode = false;
        // If in prefetch mode, we prefetch tcerts from member services to help performance
        this.preFetchMode = true;
        this.name = name;
    }
    /**
     * Get the chain name.
     * @returns The name of the chain.
     */
    Chain.prototype.getName = function () {
        return this.name;
    };
    /**
     * Add a peer given an endpoint specification.
     * @param endpoint The endpoint of the form: { url: "grpcs://host:port", tls: { .... } }
     * @returns {Peer} Returns a new peer.
     */
    Chain.prototype.addPeer = function (url, pem) {
        var peer = new Peer(url, this, pem);
        this.peers.push(peer);
        return peer;
    };
    ;
    /**
     * Get the peers for this chain.
     */
    Chain.prototype.getPeers = function () {
        return this.peers;
    };
    /**
     * Get the member whose credentials are used to register and enroll other users, or undefined if not set.
     * @param {Member} The member whose credentials are used to perform registration, or undefined if not set.
     */
    Chain.prototype.getRegistrar = function () {
        return this.registrar;
    };
    /**
     * Set the member whose credentials are used to register and enroll other users.
     * @param {Member} registrar The member whose credentials are used to perform registration.
     */
    Chain.prototype.setRegistrar = function (registrar) {
        this.registrar = registrar;
    };
    /**
     * Set the member services URL
     * @param {string} url Member services URL of the form: "grpc://host:port" or "grpcs://host:port"
     */
    Chain.prototype.setMemberServicesUrl = function (url, pem) {
        this.setMemberServices(newMemberServices(url, pem));
    };
    /**
     * Get the member service associated this chain.
     * @returns {MemberService} Return the current member service, or undefined if not set.
     */
    Chain.prototype.getMemberServices = function () {
        return this.memberServices;
    };
    ;
    /**
     * Set the member service associated this chain.  This allows the default implementation of member service to be overridden.
     */
    Chain.prototype.setMemberServices = function (memberServices) {
        this.memberServices = memberServices;
        if (memberServices instanceof MemberServicesImpl) {
            this.cryptoPrimitives = memberServices.getCrypto();
        }
    };
    ;
    /**
     * Determine if security is enabled.
     */
    Chain.prototype.isSecurityEnabled = function () {
        return this.memberServices !== undefined;
    };
    /**
     * Determine if pre-fetch mode is enabled to prefetch tcerts.
     */
    Chain.prototype.isPreFetchMode = function () {
        return this.preFetchMode;
    };
    /**
     * Set prefetch mode to true or false.
     */
    Chain.prototype.setPreFetchMode = function (preFetchMode) {
        this.preFetchMode = preFetchMode;
    };
    /**
     * Determine if dev mode is enabled.
     */
    Chain.prototype.isDevMode = function () {
        return this.devMode;
    };
    /**
     * Set dev mode to true or false.
     */
    Chain.prototype.setDevMode = function (devMode) {
        this.devMode = devMode;
    };
    /**
     * Get the key val store implementation (if any) that is currently associated with this chain.
     * @returns {KeyValStore} Return the current KeyValStore associated with this chain, or undefined if not set.
     */
    Chain.prototype.getKeyValStore = function () {
        return this.keyValStore;
    };
    /**
     * Set the key value store implementation.
     */
    Chain.prototype.setKeyValStore = function (keyValStore) {
        this.keyValStore = keyValStore;
    };
    /**
     * Get the tcert batch size.
     */
    Chain.prototype.getTCertBatchSize = function () {
        return this.tcertBatchSize;
    };
    /**
     * Set the tcert batch size.
     */
    Chain.prototype.setTCertBatchSize = function (batchSize) {
        this.tcertBatchSize = batchSize;
    };
    /**
     * Get the user member named 'name'.
     * @param cb Callback of form "function(err,Member)"
     */
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
    /**
     * Get a user.
     * A user is a specific type of member.
     * Another type of member is a peer.
     */
    Chain.prototype.getUser = function (name, cb) {
        return this.getMember(name, cb);
    };
    // Try to get the member from cache.
    // If not found, create a new one, restore the state if found, and then store in cache.
    Chain.prototype.getMemberHelper = function (name, cb) {
        var self = this;
        // Try to get the member state from the cache
        var member = self.members[name];
        if (member)
            return cb(null, member);
        // Create the member and try to restore it's state from the key value store (if found).
        member = new Member(name, self);
        member.restoreState(function (err) {
            if (err)
                return cb(err);
            cb(null, member);
        });
    };
    /**
     * Register a user or other member type with the chain.
     * @param registrationRequest Registration information.
     * @param cb Callback with registration results
     */
    Chain.prototype.register = function (registrationRequest, cb) {
        var self = this;
        self.getMember(registrationRequest.enrollmentID, function (err, member) {
            if (err)
                return cb(err);
            member.register(registrationRequest, cb);
        });
    };
    /**
     * Enroll a user or other identity which has already been registered.
     * If the user has already been enrolled, this will still succeed.
     * @param name The name of the user or other member to enroll.
     * @param secret The secret of the user or other member to enroll.
     * @param cb The callback to return the user or other member.
     */
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
    /**
     * Register and enroll a user or other member type.
     * This assumes that a registrar with sufficient privileges has been set.
     * @param registrationRequest Registration information.
     * @params
     */
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
    /**
     * Send a transaction to a peer.
     * @param tx A transaction
     * @param eventEmitter An event emitter
     */
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
/**
 * A member is an entity that transacts on a chain.
 * Types of members include end users, peers, etc.
 */
var Member = (function () {
    /**
     * Constructor for a member.
     * @param cfg {string | RegistrationRequest} The member name or registration request.
     * @returns {Member} A member who is neither registered nor enrolled.
     */
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
    /**
     * Get the member name.
     * @returns {string} The member name.
     */
    Member.prototype.getName = function () {
        return this.name;
    };
    /**
     * Get the chain.
     * @returns {Chain} The chain.
     */
    Member.prototype.getChain = function () {
        return this.chain;
    };
    ;
    /**
     * Get the roles.
     * @returns {string[]} The roles.
     */
    Member.prototype.getRoles = function () {
        return this.roles;
    };
    ;
    /**
     * Set the roles.
     * @param roles {string[]} The roles.
     */
    Member.prototype.setRoles = function (roles) {
        this.roles = roles;
    };
    ;
    /**
     * Get the account.
     * @returns {string} The account.
     */
    Member.prototype.getAccount = function () {
        return this.account;
    };
    ;
    /**
     * Set the account.
     * @param account The account.
     */
    Member.prototype.setAccount = function (account) {
        this.account = account;
    };
    ;
    /**
     * Get the affiliation.
     * @returns {string} The affiliation.
     */
    Member.prototype.getAffiliation = function () {
        return this.affiliation;
    };
    ;
    /**
     * Set the affiliation.
     * @param affiliation The affiliation.
     */
    Member.prototype.setAffiliation = function (affiliation) {
        this.affiliation = affiliation;
    };
    ;
    /**
     * Get the transaction certificate (tcert) batch size, which is the number of tcerts retrieved
     * from member services each time (i.e. in a single batch).
     * @returns The tcert batch size.
     */
    Member.prototype.getTCertBatchSize = function () {
        if (this.tcertBatchSize === undefined) {
            return this.chain.getTCertBatchSize();
        }
        else {
            return this.tcertBatchSize;
        }
    };
    /**
     * Set the transaction certificate (tcert) batch size.
     * @param batchSize
     */
    Member.prototype.setTCertBatchSize = function (batchSize) {
        this.tcertBatchSize = batchSize;
    };
    /**
     * Get the enrollment info.
     * @returns {Enrollment} The enrollment.
     */
    Member.prototype.getEnrollment = function () {
        return this.enrollment;
    };
    ;
    /**
     * Determine if this name has been registered.
     * @returns {boolean} True if registered; otherwise, false.
     */
    Member.prototype.isRegistered = function () {
        return this.enrollmentSecret !== undefined;
    };
    /**
     * Determine if this name has been enrolled.
     * @returns {boolean} True if enrolled; otherwise, false.
     */
    Member.prototype.isEnrolled = function () {
        return this.enrollment !== undefined;
    };
    /**
     * Register the member.
     * @param cb Callback of the form: {function(err,enrollmentSecret)}
     */
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
    /**
     * Enroll the member and return the enrollment results.
     * @param enrollmentSecret The password or enrollment secret as returned by register.
     * @param cb Callback to report an error if it occurs
     */
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
            // Generate queryStateKey
            self.enrollment.queryStateKey = self.chain.cryptoPrimitives.generateNonce();
            // Save state
            self.saveState(function (err) {
                if (err)
                    return cb(err);
                // Unmarshall chain key
                // TODO: during restore, unmarshall enrollment.chainKey
                debug("[memberServices.enroll] Unmarshalling chainKey");
                var ecdsaChainKey = self.chain.cryptoPrimitives.ecdsaPEMToPublicKey(self.enrollment.chainKey);
                self.enrollment.enrollChainKey = ecdsaChainKey;
                cb(null, enrollment);
            });
        });
    };
    /**
     * Perform both registration and enrollment.
     * @param cb Callback of the form: {function(err,{key,cert,chainKey})}
     */
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
    /**
     * Issue a deploy request on behalf of this member.
     * @param deployRequest {Object}
     * @returns {TransactionContext} Emits 'submitted', 'complete', and 'error' events.
     */
    Member.prototype.deploy = function (deployRequest) {
        var tx = this.newTransactionContext();
        tx.deploy(deployRequest);
        return tx;
    };
    /**
     * Issue a invoke request on behalf of this member.
     * @param invokeRequest {Object}
     * @returns {TransactionContext} Emits 'submitted', 'complete', and 'error' events.
     */
    Member.prototype.invoke = function (invokeRequest) {
        var tx = this.newTransactionContext();
        tx.invoke(invokeRequest);
        return tx;
    };
    /**
     * Issue a query request on behalf of this member.
     * @param queryRequest {Object}
     * @returns {TransactionContext} Emits 'submitted', 'complete', and 'error' events.
     */
    Member.prototype.query = function (queryRequest) {
        var tx = this.newTransactionContext();
        tx.query(queryRequest);
        return tx;
    };
    /**
     * Create a transaction context with which to issue build, deploy, invoke, or query transactions.
     * Only call this if you want to use the same tcert for multiple transactions.
     * @param {Object} tcert A transaction certificate from member services.  This is optional.
     * @returns A transaction context.
     */
    Member.prototype.newTransactionContext = function (tcert) {
        return new TransactionContext(this, tcert);
    };
    Member.prototype.getUserCert = function (cb) {
        this.getNextTCert(cb);
    };
    /**
     * Get the next available transaction certificate.
     * @param cb
     */
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
    // Determine if we should issue a request to get more tcerts now.
    Member.prototype.shouldGetTCerts = function () {
        var self = this;
        // Do nothing if we are already getting more tcerts
        if (self.gettingTCerts) {
            debug("shouldGetTCerts: no, already getting tcerts");
            return false;
        }
        // If there are none, then definitely get more
        if (self.tcerts.length == 0) {
            debug("shouldGetTCerts: yes, we have no tcerts");
            return true;
        }
        // If we aren't in prefetch mode, return false;
        if (!self.chain.isPreFetchMode()) {
            debug("shouldGetTCerts: no, prefetch disabled");
            return false;
        }
        // Otherwise, see if we should prefetch based on the arrival rate
        // (i.e. the rate at which tcerts are requested) and the response
        // time.
        // 'arrivalRate' is in req/ms and 'responseTime' in ms,
        // so 'tcertCountThreshold' is number of tcerts at which we should
        // request the next batch of tcerts so we don't have to wait on the
        // transaction path.  Note that we add 1 sec to the average response
        // time to add a little buffer time so we don't have to wait.
        var arrivalRate = self.arrivalRate.getValue();
        var responseTime = self.getTCertResponseTime.getValue() + 1000;
        var tcertThreshold = arrivalRate * responseTime;
        var tcertCount = self.tcerts.length;
        var result = tcertCount <= tcertThreshold;
        debug(util.format("shouldGetTCerts: %s, threshold=%s, count=%s, rate=%s, responseTime=%s", result, tcertThreshold, tcertCount, arrivalRate, responseTime));
        return result;
    };
    // Call member services to get more tcerts
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
                // Error all waiters
                while (self.getTCertWaiters.length > 0) {
                    self.getTCertWaiters.shift()(err);
                }
                return;
            }
            self.getTCertResponseTime.stop();
            // Add to member's tcert list
            while (tcerts.length > 0) {
                self.tcerts.push(tcerts.shift());
            }
            // Allow waiters to proceed
            while (self.getTCertWaiters.length > 0 && self.tcerts.length > 0) {
                self.getTCertWaiters.shift()(null, self.tcerts.shift());
            }
        });
    };
    /**
     * Save the state of this member to the key value store.
     * @param cb Callback of the form: {function(err}
     */
    Member.prototype.saveState = function (cb) {
        var self = this;
        self.keyValStore.setValue(self.keyValStoreName, self.toString(), cb);
    };
    /**
     * Restore the state of this member from the key value store (if found).  If not found, do nothing.
     * @param cb Callback of the form: function(err}
     */
    Member.prototype.restoreState = function (cb) {
        var self = this;
        self.keyValStore.getValue(self.keyValStoreName, function (err, memberStr) {
            if (err)
                return cb(err);
            // debug("restoreState: name=%s, memberStr=%s", self.getName(), memberStr);
            if (memberStr) {
                // The member was found in the key value store, so restore the state.
                self.fromString(memberStr);
            }
            cb(null);
        });
    };
    /**
     * Get the current state of this member as a string
     * @return {string} The state of this member as a string
     */
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
    /**
     * Save the current state of this member as a string
     * @return {string} The state of this member as a string
     */
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
/**
 * A transaction context emits events 'submitted', 'complete', and 'error'.
 * Each transaction context uses exactly one tcert.
 */
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
    /**
     * Get the member with which this transaction context is associated.
     * @returns The member
     */
    TransactionContext.prototype.getMember = function () {
        return this.member;
    };
    /**
     * Get the chain with which this transaction context is associated.
     * @returns The chain
     */
    TransactionContext.prototype.getChain = function () {
        return this.chain;
    };
    ;
    /**
     * Get the member services, or undefined if security is not enabled.
     * @returns The member services
     */
    TransactionContext.prototype.getMemberServices = function () {
        return this.memberServices;
    };
    ;
    /**
     * Issue a deploy transaction.
     * @param deployRequest {Object} A deploy request of the form: { chaincodeID, payload, metadata, uuid, timestamp, confidentiality: { level, version, nonce }
   */
    TransactionContext.prototype.deploy = function (deployRequest) {
        var self = this;
        debug("received deploy request: %j", deployRequest);
        self.getMyTCert(function (err) {
            if (err) {
                debug('Failed getting a new TCert [%s]', err);
                self.emit('error', err);
                return self;
            }
            return self.execute(self.newBuildOrDeployTransaction(deployRequest, false));
        });
        return self;
    };
    /**
     * Issue an invoke transaction.
     * @param invokeRequest {Object} An invoke request of the form: XXX
     */
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
    /**
     * Issue an query transaction.
     * @param queryRequest {Object} A query request of the form: XXX
     */
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
    /**
     * Execute a transaction
     * @param tx {Transaction} The transaction.
     */
    TransactionContext.prototype.execute = function (tx) {
        debug('Executing transaction [%j]', tx);
        var self = this;
        // Get the TCert
        self.getMyTCert(function (err, tcert) {
            if (err) {
                debug('Failed getting a new TCert [%s]', err);
                return self.emit('error', err);
            }
            if (tcert) {
                // Set nonce
                tx.setNonce(self.nonce);
                // Process confidentiality
                debug('Process Confidentiality...');
                self.processConfidentiality(tx);
                debug('Sign transaction...');
                // Add the tcert
                tx.setCert(tcert.publicKey);
                // sign the transaction bytes
                var txBytes = tx.toBuffer();
                var derSignature = self.chain.cryptoPrimitives.ecdsaSign(tcert.privateKey.getPrivate('hex'), txBytes).toDER();
                // debug('signature: ', derSignature);
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
        // is confidentiality required?
        if (transaction.getConfidentialityLevel() != _fabricProto.ConfidentialityLevel.CONFIDENTIAL) {
            // No confidentiality is required
            return;
        }
        debug('Process Confidentiality ...');
        var self = this;
        // Set confidentiality level and protocol version
        transaction.setConfidentialityProtocolVersion('1.2');
        // Generate transaction key. Common to all type of transactions
        var txKey = self.chain.cryptoPrimitives.eciesKeyGen();
        debug('txkey [%j]', txKey.pubKeyObj.pubKeyHex);
        debug('txKey.prvKeyObj %j', txKey.prvKeyObj.toString());
        var privBytes = self.chain.cryptoPrimitives.ecdsaPrivateKeyToASN1(txKey.prvKeyObj.prvKeyHex);
        debug('privBytes %s', privBytes.toString());
        // Generate stateKey. Transaction type dependent step.
        var stateKey;
        if (transaction.getType() == _fabricProto.Transaction.Type.CHAINCODE_DEPLOY) {
            // The request is for a deploy
            stateKey = new Buffer(self.chain.cryptoPrimitives.aesKeyGen());
        }
        else if (transaction.getType() == _fabricProto.Transaction.Type.CHAINCODE_INVOKE) {
            // The request is for an execute
            // Empty state key
            stateKey = new Buffer([]);
        }
        else {
            // The request is for a query
            debug('Generate state key...');
            stateKey = new Buffer(self.chain.cryptoPrimitives.hmacAESTruncated(self.member.getEnrollment().queryStateKey, [CONFIDENTIALITY_1_2_STATE_KD_C6].concat(self.nonce)));
        }
        // Prepare ciphertexts
        // Encrypts message to validators using self.enrollChainKey
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
        // Encrypts chaincodeID using txKey
        // debug('CHAINCODE ID %j', transaction.chaincodeID);
        var encryptedChaincodeID = self.chain.cryptoPrimitives.eciesEncrypt(txKey.pubKeyObj, transaction.getChaincodeID().buffer);
        transaction.setChaincodeID(encryptedChaincodeID);
        // Encrypts payload using txKey
        // debug('PAYLOAD ID %j', transaction.payload);
        var encryptedPayload = self.chain.cryptoPrimitives.eciesEncrypt(txKey.pubKeyObj, transaction.getPayload().buffer);
        transaction.setPayload(encryptedPayload);
        // Encrypt metadata using txKey
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
    /**
     * Create a deploy transaction.
     * @param request {Object} A BuildRequest or DeployRequest
     */
    TransactionContext.prototype.newBuildOrDeployTransaction = function (request, isBuildRequest) {
        var self = this;
        var tx = new _fabricProto.Transaction();
        if (isBuildRequest) {
            tx.setType(_fabricProto.Transaction.Type.CHAINCODE_BUILD);
        }
        else {
            tx.setType(_fabricProto.Transaction.Type.CHAINCODE_DEPLOY);
        }
        // Set the chaincodeID
        var chaincodeID = new _chaincodeProto.ChaincodeID();
        chaincodeID.setName(request.chaincodeID);
        debug("newBuildOrDeployTransaction: chaincodeID: " + JSON.stringify(chaincodeID));
        tx.setChaincodeID(chaincodeID.toBuffer());
        // Construct the ChaincodeSpec
        var chaincodeSpec = new _chaincodeProto.ChaincodeSpec();
        // Set Type -- GOLANG is the only chaincode language supported at this time
        chaincodeSpec.setType(_chaincodeProto.ChaincodeSpec.Type.GOLANG);
        // Set chaincodeID
        chaincodeSpec.setChaincodeID(chaincodeID);
        // Set ctorMsg
        var chaincodeInput = new _chaincodeProto.ChaincodeInput();
        chaincodeInput.setFunction(request.fcn);
        chaincodeInput.setArgs(request.args);
        chaincodeSpec.setCtorMsg(chaincodeInput);
        // Construct the ChaincodeDeploymentSpec (i.e. the payload)
        var chaincodeDeploymentSpec = new _chaincodeProto.ChaincodeDeploymentSpec();
        chaincodeDeploymentSpec.setChaincodeSpec(chaincodeSpec);
        tx.setPayload(chaincodeDeploymentSpec.toBuffer());
        // Set the transaction UUID
        if (self.chain.isDevMode()) {
            tx.setUuid(request.chaincodeID);
        }
        else {
            tx.setUuid(generateUUID());
        }
        // Set the transaction timestamp
        tx.setTimestamp(generateTimestamp());
        // Set confidentiality level
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
        return tx;
    };
    /**
     * Create an invoke or query transaction.
     * @param request {Object} A build or deploy request of the form: { chaincodeID, payload, metadata, uuid, timestamp, confidentiality: { level, version, nonce }
     */
    TransactionContext.prototype.newInvokeOrQueryTransaction = function (request, isInvokeRequest) {
        var self = this;
        // Create a deploy transaction
        var tx = new _fabricProto.Transaction();
        if (isInvokeRequest) {
            tx.setType(_fabricProto.Transaction.Type.CHAINCODE_INVOKE);
        }
        else {
            tx.setType(_fabricProto.Transaction.Type.CHAINCODE_QUERY);
        }
        // Set the chaincodeID
        var chaincodeID = new _chaincodeProto.ChaincodeID();
        chaincodeID.setName(request.chaincodeID);
        debug("newInvokeOrQueryTransaction: request=%j, chaincodeID=%s", request, JSON.stringify(chaincodeID));
        tx.setChaincodeID(chaincodeID.toBuffer());
        // Construct the ChaincodeSpec
        var chaincodeSpec = new _chaincodeProto.ChaincodeSpec();
        // Set Type -- GOLANG is the only chaincode language supported at this time
        chaincodeSpec.setType(_chaincodeProto.ChaincodeSpec.Type.GOLANG);
        // Set chaincodeID
        chaincodeSpec.setChaincodeID(chaincodeID);
        // Set ctorMsg
        var chaincodeInput = new _chaincodeProto.ChaincodeInput();
        chaincodeInput.setFunction(request.fcn);
        chaincodeInput.setArgs(request.args);
        chaincodeSpec.setCtorMsg(chaincodeInput);
        // Construct the ChaincodeInvocationSpec (i.e. the payload)
        var chaincodeInvocationSpec = new _chaincodeProto.ChaincodeInvocationSpec();
        chaincodeInvocationSpec.setChaincodeSpec(chaincodeSpec);
        tx.setPayload(chaincodeInvocationSpec.toBuffer());
        // Set the transaction UUID
        tx.setUuid(generateUUID());
        // Set the transaction timestamp
        tx.setTimestamp(generateTimestamp());
        // Set confidentiality level
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
            // cert based
            var certRaw = new Buffer(self.tcert.publicKey);
            // debug('========== Invoker Cert [%s]', certRaw.toString('hex'));
            var nonceRaw = new Buffer(self.nonce);
            var bindingMsg = Buffer.concat([certRaw, nonceRaw]);
            // debug('========== Binding Msg [%s]', bindingMsg.toString('hex'));
            this.binding = new Buffer(self.chain.cryptoPrimitives.hash(bindingMsg), 'hex');
            // debug('========== Binding [%s]', this.binding.toString('hex'));
            var ctor = chaincodeSpec.getCtorMsg().toBuffer();
            // debug('========== Ctor [%s]', ctor.toString('hex'));
            var txmsg = Buffer.concat([ctor, this.binding]);
            // debug('========== Pyaload||binding [%s]', txmsg.toString('hex'));
            var mdsig = self.chain.cryptoPrimitives.ecdsaSign(request.userCert.privateKey.getPrivate('hex'), txmsg);
            var sigma = new Buffer(mdsig.toDER());
            // debug('========== Sigma [%s]', sigma.toString('hex'));
            tx.setMetadata(sigma);
        }
        return tx;
    };
    return TransactionContext;
}(events.EventEmitter));
exports.TransactionContext = TransactionContext; // end TransactionContext
/**
 * The Peer class represents a peer to which HLC sends deploy, invoke, or query requests.
 */
var Peer = (function () {
    /**
     * Constructor for a peer given the endpoint config for the peer.
     * @param {string} url The URL of
     * @param {Chain} The chain of which this peer is a member.
     * @returns {Peer} The new peer.
     */
    function Peer(url, chain, pem) {
        /**
         * Send a transaction to this peer.
         * @param tx A transaction
         * @param eventEmitter The event emitter
         */
        this.sendTransaction = function (tx, eventEmitter) {
            var self = this;
            //debug("peer.sendTransaction: sending %j", tx);
            // Send the transaction to the peer node via grpc
            // The rpc specification on the peer side is:
            //     rpc ProcessTransaction(Transaction) returns (Response) {}
            self.peerClient.processTransaction(tx, function (err, response) {
                if (err) {
                    debug("peer.sendTransaction: error=%j", err);
                    return eventEmitter.emit('error', err);
                }
                debug("peer.sendTransaction: received %j", response);
                // Check transaction type here, as deploy/invoke are asynchronous calls,
                // whereas a query is a synchonous call. As such, deploy/invoke will emit
                // 'submitted' and 'error', while a query will emit 'complete' and 'error'.
                var txType = tx.getType();
                switch (txType) {
                    case _fabricProto.Transaction.Type.CHAINCODE_DEPLOY:
                        if (response.status === "SUCCESS") {
                            // Deploy transaction has been submitted
                            if (!response.msg || response.msg === "") {
                                eventEmitter.emit('error', 'the response is missing the transaction UUID');
                            }
                            else {
                                eventEmitter.emit('submitted', response.msg);
                                self.waitToComplete(eventEmitter);
                            }
                        }
                        else {
                            // Deploy completed with status "FAILURE" or "UNDEFINED"
                            eventEmitter.emit('error', response.msg);
                        }
                        break;
                    case _fabricProto.Transaction.Type.CHAINCODE_INVOKE:
                        if (response.status === "SUCCESS") {
                            // Invoke transaction has been submitted
                            eventEmitter.emit('submitted', response.msg.toString());
                            self.waitToComplete(eventEmitter);
                        }
                        else {
                            // Invoke completed with status "FAILURE" or "UNDEFINED"
                            eventEmitter.emit('error', response.msg);
                        }
                        break;
                    case _fabricProto.Transaction.Type.CHAINCODE_QUERY:
                        if (response.status === "SUCCESS") {
                            // Query transaction has been completed
                            eventEmitter.emit('complete', response.msg);
                        }
                        else {
                            // Query completed with status "FAILURE" or "UNDEFINED"
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
    /**
     * Get the chain of which this peer is a member.
     * @returns {Chain} The chain of which this peer is a member.
     */
    Peer.prototype.getChain = function () {
        return this.chain;
    };
    /**
     * Get the URL of the peer.
     * @returns {string} Get the URL associated with the peer.
     */
    Peer.prototype.getUrl = function () {
        return this.url;
    };
    /**
     * For now, just wait 5 seconds and then fire the complete event.
     * This is a temporary hack until event notification is implemented.
     * TODO: implement this appropriately.
     */
    Peer.prototype.waitToComplete = function (eventEmitter) {
        debug("waiting 5 seconds before emitting complete event");
        var emitComplete = function () {
            debug("emitting completion event");
            eventEmitter.emit('complete');
        };
        setTimeout(emitComplete, 5000);
    };
    /**
     * Remove the peer from the chain.
     */
    Peer.prototype.remove = function () {
        throw Error("TODO: implement");
    };
    return Peer;
}());
exports.Peer = Peer; // end Peer
/**
 * An endpoint currently takes only URL (currently).
 * @param url
 */
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
/**
 * MemberServicesImpl is the default implementation of a member services client.
 */
var MemberServicesImpl = (function () {
    /**
     * MemberServicesImpl constructor
     * @param config The config information required by this member services implementation.
     * @returns {MemberServices} A MemberServices object.
     */
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
    /**
     * Get the security level
     * @returns The security level
     */
    MemberServicesImpl.prototype.getSecurityLevel = function () {
        return this.cryptoPrimitives.getSecurityLevel();
    };
    /**
     * Set the security level
     * @params securityLevel The security level
     */
    MemberServicesImpl.prototype.setSecurityLevel = function (securityLevel) {
        this.cryptoPrimitives.setSecurityLevel(securityLevel);
    };
    /**
     * Get the hash algorithm
     * @returns {string} The hash algorithm
     */
    MemberServicesImpl.prototype.getHashAlgorithm = function () {
        return this.cryptoPrimitives.getHashAlgorithm();
    };
    /**
     * Set the hash algorithm
     * @params hashAlgorithm The hash algorithm ('SHA2' or 'SHA3')
     */
    MemberServicesImpl.prototype.setHashAlgorithm = function (hashAlgorithm) {
        this.cryptoPrimitives.setHashAlgorithm(hashAlgorithm);
    };
    MemberServicesImpl.prototype.getCrypto = function () {
        return this.cryptoPrimitives;
    };
    /**
     * Register the member and return an enrollment secret.
     * @param req Registration request with the following fields: name, role
     * @param registrar The identity of the registrar (i.e. who is performing the registration)
     * @param cb Callback of the form: {function(err,enrollmentSecret)}
     */
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
        // Create registrar info
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
        // Sign the registration request
        var buf = protoReq.toBuffer();
        var signKey = self.cryptoPrimitives.ecdsaKeyFromPrivate(registrar.getEnrollment().key, 'hex');
        var sig = self.cryptoPrimitives.ecdsaSign(signKey, buf);
        protoReq.setSig(new _caProto.Signature({
            type: _caProto.CryptoType.ECDSA,
            r: new Buffer(sig.r.toString()),
            s: new Buffer(sig.s.toString())
        }));
        // Send the registration request
        self.ecaaClient.registerUser(protoReq, function (err, token) {
            debug("register %j: err=%j, token=%s", protoReq, err, token);
            if (cb)
                return cb(err, token ? token.tok.toString() : null);
        });
    };
    /**
     * Enroll the member and return an opaque member object
     * @param req Enrollment request with the following fields: name, enrollmentSecret
     * @param cb Callback of the form: {function(err,{key,cert,chainKey})}
     */
    MemberServicesImpl.prototype.enroll = function (req, cb) {
        var self = this;
        cb = cb || nullCB;
        debug("[MemberServicesImpl.enroll] [%j]", req);
        if (!req.enrollmentID)
            return cb(Error("req.enrollmentID is not set"));
        if (!req.enrollmentSecret)
            return cb(Error("req.enrollmentSecret is not set"));
        debug("[MemberServicesImpl.enroll] Generating keys...");
        // generate ECDSA keys: signing and encryption keys
        // 1) signing key
        var signingKeyPair = self.cryptoPrimitives.ecdsaKeyGen();
        var spki = new asn1.x509.SubjectPublicKeyInfo(signingKeyPair.pubKeyObj);
        // 2) encryption key
        var encryptionKeyPair = self.cryptoPrimitives.ecdsaKeyGen();
        var spki2 = new asn1.x509.SubjectPublicKeyInfo(encryptionKeyPair.pubKeyObj);
        debug("[MemberServicesImpl.enroll] Generating keys...done!");
        // create the proto message
        var eCertCreateRequest = new _caProto.ECertCreateReq();
        var timestamp = new _timeStampProto({ seconds: Date.now() / 1000, nanos: 0 });
        eCertCreateRequest.setTs(timestamp);
        eCertCreateRequest.setId({ id: req.enrollmentID });
        eCertCreateRequest.setTok({ tok: new Buffer(req.enrollmentSecret) });
        debug("[MemberServicesImpl.enroll] Generating request! %j", spki.getASN1Object().getEncodedHex());
        // public signing key (ecdsa)
        var signPubKey = new _caProto.PublicKey({
            type: _caProto.CryptoType.ECDSA,
            key: new Buffer(spki.getASN1Object().getEncodedHex(), 'hex')
        });
        eCertCreateRequest.setSign(signPubKey);
        debug("[MemberServicesImpl.enroll] Adding signing key!");
        // public encryption key (ecdsa)
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
            //debug(decryptedTokBytes);
            // debug(decryptedTokBytes.toString());
            // debug('decryptedTokBytes [%s]', decryptedTokBytes.toString());
            eCertCreateRequest.setTok({ tok: decryptedTokBytes });
            eCertCreateRequest.setSig(null);
            var buf = eCertCreateRequest.toBuffer();
            var signKey = self.cryptoPrimitives.ecdsaKeyFromPrivate(signingKeyPair.prvKeyObj.prvKeyHex, 'hex');
            //debug(new Buffer(sha3_384(buf),'hex'));
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
                // debug('cert:\n\n',enrollment.cert)
                cb(null, enrollment);
            });
        });
    }; // end enroll
    /**
     * Get an array of transaction certificates (tcerts).
     * @param {Object} req Request of the form: {name,enrollment,num} where
     * 'name' is the member name,
     * 'enrollment' is what was returned by enroll, and
     * 'num' is the number of transaction contexts to obtain.
     * @param {function(err,[Object])} cb The callback function which is called with an error as 1st arg and an array of tcerts as 2nd arg.
     */
    MemberServicesImpl.prototype.getTCertBatch = function (req, cb) {
        var self = this;
        cb = cb || nullCB;
        var timestamp = new _timeStampProto({ seconds: Date.now() / 1000, nanos: 0 });
        // create the proto
        var tCertCreateSetReq = new _caProto.TCertCreateSetReq();
        tCertCreateSetReq.setTs(timestamp);
        tCertCreateSetReq.setId({ id: req.name });
        tCertCreateSetReq.setNum(req.num);
        // serialize proto
        var buf = tCertCreateSetReq.toBuffer();
        // sign the transaction using enrollment key
        var signKey = self.cryptoPrimitives.ecdsaKeyFromPrivate(req.enrollment.key, 'hex');
        var sig = self.cryptoPrimitives.ecdsaSign(signKey, buf);
        tCertCreateSetReq.setSig(new _caProto.Signature({
            type: _caProto.CryptoType.ECDSA,
            r: new Buffer(sig.r.toString()),
            s: new Buffer(sig.s.toString())
        }));
        // send the request
        self.tcapClient.createCertificateSet(tCertCreateSetReq, function (err, resp) {
            if (err)
                return cb(err);
            // debug('tCertCreateSetResp:\n', resp);
            cb(null, self.processTCertBatch(req, resp));
        });
    };
    /**
     * Process a batch of tcerts after having retrieved them from the TCA.
     */
    MemberServicesImpl.prototype.processTCertBatch = function (req, resp) {
        var self = this;
        //
        // Derive secret keys for TCerts
        //
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
        // Loop through certs and extract private keys
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
            // debug("HERE2: got x509 cert");
            // extract the encrypted bytes from extension attribute
            var tCertIndexCT = x509Certificate.criticalExtension(crypto.TCertEncTCertIndex);
            // debug('tCertIndexCT: ',JSON.stringify(tCertIndexCT));
            var tCertIndex = self.cryptoPrimitives.aesCBCPKCS7Decrypt(tCertOwnerEncryptKey, tCertIndexCT);
            // debug('tCertIndex: ',JSON.stringify(tCertIndex));
            var expansionValue = self.cryptoPrimitives.hmac(expansionKey, tCertIndex);
            // debug('expansionValue: ',expansionValue);
            // compute the private key
            var one = new BN(1);
            var k = new BN(expansionValue);
            var n = self.cryptoPrimitives.ecdsaKeyFromPrivate(enrollKey, 'hex').ec.curve.n.sub(one);
            k = k.mod(n).add(one);
            var D = self.cryptoPrimitives.ecdsaKeyFromPrivate(enrollKey, 'hex').getPrivate().add(k);
            var pubHex = self.cryptoPrimitives.ecdsaKeyFromPrivate(enrollKey, 'hex').getPublic('hex');
            D = D.mod(self.cryptoPrimitives.ecdsaKeyFromPublic(pubHex, 'hex').ec.curve.n);
            // Put private and public key in returned tcert
            var tcert = new TCert(tCert.cert, self.cryptoPrimitives.ecdsaKeyFromPrivate(D, 'hex'));
            tCertBatch.push(tcert);
        }
        if (tCertBatch.length == 0) {
            throw Error('Failed fetching TCertBatch. No valid TCert received.');
        }
        return tCertBatch;
    }; // end processTCertBatch
    return MemberServicesImpl;
}()); // end MemberServicesImpl
function newMemberServices(url, pem) {
    return new MemberServicesImpl(url, pem);
}
/**
 * A local file-based key value store.
 * This implements the KeyValStore interface.
 */
var FileKeyValStore = (function () {
    function FileKeyValStore(dir) {
        /**
         * Set the value associated with name.
         * @param name
         * @param cb function(err)
         */
        this.setValue = function (name, value, cb) {
            var path = this.dir + '/' + name;
            fs.writeFile(path, value, cb);
        };
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
}()); // end FileKeyValStore
/**
 * generateUUID returns an RFC4122 compliant UUID.
 *    http://www.ietf.org/rfc/rfc4122.txt
 */
function generateUUID() {
    return uuid.v4();
}
;
/**
 * generateTimestamp returns the current time in the google/protobuf/timestamp.proto structure.
 */
function generateTimestamp() {
    return new _timeStampProto({ seconds: Date.now() / 1000, nanos: 0 });
}
function toKeyValStoreName(name) {
    return "member." + name;
}
/**
 * Create and load peers for bluemix.
 */
function bluemixInit() {
    var vcap = process.env['VCAP_SERVICES'];
    if (!vcap)
        return false; // not in bluemix
    // TODO: Take logic from marbles app
    return true;
}
function nullCB() {
}
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
    // TODO: find ambient definition for url
    var purl = urlParser.parse(url, true);
    var protocol = purl.protocol;
    if (endsWith(protocol, ":")) {
        purl.protocol = protocol.slice(0, -1);
    }
    return purl;
}
// Convert a list of member type names to the role mask currently used by the peer
function rolesToMask(roles) {
    var mask = 0;
    if (roles) {
        for (var role in roles) {
            switch (role) {
                case 'client':
                    mask |= 1;
                    break; // Client mask
                case 'peer':
                    mask |= 2;
                    break; // Peer mask
                case 'validator':
                    mask |= 4;
                    break; // Validator mask
                case 'auditor':
                    mask |= 8;
                    break; // Auditor mask
            }
        }
    }
    if (mask === 0)
        mask = 1; // Client
    return mask;
}
function endsWith(str, suffix) {
    return str.length >= suffix.length && str.substr(str.length - suffix.length) === suffix;
}
;
/**
 * Create a new chain.  If it already exists, throws an Error.
 * @param name {string} Name of the chain.  It can be any name and has value only for the client.
 * @returns
 */
function newChain(name) {
    var chain = _chains[name];
    if (chain)
        throw Error(util.format("chain %s already exists", name));
    chain = new Chain(name);
    _chains[name] = chain;
    return chain;
}
exports.newChain = newChain;
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
}
exports.getChain = getChain;
/**
 * Create an instance of a FileKeyValStore.
 */
function newFileKeyValStore(dir) {
    return new FileKeyValStore(dir);
}
exports.newFileKeyValStore = newFileKeyValStore;
//# sourceMappingURL=hlc.js.map