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
import * as crypto from "./crypto"
import events = require('events');

let debug = debugModule('hlc');   // 'hlc' stands for 'HyperLedger Client'
let KEYUTIL = jsrsa.KEYUTIL;
let asn1 = jsrsa.asn1;
let sha3_256 = sha3.sha3_256;
let sha3_384 = sha3.sha3_384;
var asn1Builder = require('asn1');

let _caProto = grpc.load(__dirname + "/protos/ca.proto").protos;
let _fabricProto = grpc.load(__dirname + "/protos/fabric.proto").protos;
let _timeStampProto = grpc.load(__dirname + "/protos/google/protobuf/timestamp.proto").google.protobuf.Timestamp;
let _chaincodeProto = grpc.load(__dirname + "/protos/chaincode.proto").protos;

let DEFAULT_SECURITY_LEVEL = 256;
let DEFAULT_HASH_ALGORITHM = "SHA3";
let CONFIDENTIALITY_1_2_STATE_KD_C6 = 6;

let _chains = {};

/**
 * The KeyValStore interface used for persistent storage.
 */
export interface KeyValStore {

    /**
     * Get the value associated with name.
     * @param name
     * @param cb function(err,value)
     */
    getValue(name:string, cb:GetValueCallback):void;

    /**
     * Set the value associated with name.
     * @param name
     * @param cb function(err)
     */
    setValue(name:string, value:string, cb:ErrorCallback);

}

export interface MemberServices {

    /**
     * Get the security level
     * @returns The security level
     */
    getSecurityLevel():number;

    /**
     * Set the security level
     * @params securityLevel The security level
     */
    setSecurityLevel(securityLevel:number):void;

    /**
     * Get the hash algorithm
     * @returns The security level
     */
    getHashAlgorithm():string;

    /**
     * Set the security level
     * @params securityLevel The security level
     */
    setHashAlgorithm(hashAlgorithm:string):void;

    /**
     * Register the member and return an enrollment secret.
     * @param req Registration request with the following fields: name, role
     * @param cb Callback of the form: {function(err,enrollmentSecret)}
     */
    register(req:RegistrationRequest, cb:RegisterCallback):void;

    /**
     * Enroll the member and return an opaque member object
     * @param req Enrollment request with the following fields: name, enrollmentSecret
     * @param cb Callback to report an error if it occurs.
     */
    enroll(req:EnrollmentRequest, cb:EnrollCallback):void;

    /**
     * Get an array of transaction certificates (tcerts).
     * @param {Object} req Request of the form: {name,enrollment,num} where
     * 'name' is the member name,
     * 'enrollment' is what was returned by enroll, and
     * 'num' is the number of transaction contexts to obtain.
     * @param {function(err,[Object])} cb The callback function which is called with an error as 1st arg and an array of tcerts as 2nd arg.
     */
    getTCerts(req:{name:string, enrollment:Enrollment, num:number}, cb:GetTCertsCallback):void;

}

export interface RegistrationRequest {
    enrollmentID:string;
    roles?:string[];
    account:string;
    affiliation:string;
    // 'registrar' enables this identity to register other members with roles 'roles'
    // and can delegate the 'delegationRoles' roles
    registrar?:{ roles:string[], delegationRoles?:string[] };
}

export interface EnrollmentRequest {
    enrollmentID:string;
    enrollmentSecret:string;
}

export interface GetMemberCallback { (err:Error, member?:Member):void
}

export interface RegisterCallback {(err:Error, enrollmentPassword?:string):void
}

export interface EnrollCallback {(err:Error, enrollment?:Enrollment):void;}

export interface Enrollment {
    key:Buffer;
    cert:string;
    chainKey:string;
}

export interface GetTCertsRequest {
    name:string;
    enrollment:Enrollment; // Return value from MemberServices.enroll
    num:number;       // Number of tcerts to retrieve
}

export interface GetTCertsCallback { (err:Error, tcerts?:TCert[]):void
}

export interface GetTCertCallback { (err:Error, tcert?:TCert):void
}

export interface GetTCertCallback { (err:Error, tcert?:TCert):void
}

export enum PrivacyLevel {
    Nominal = 0,
    Anonymous = 1
}

export class Certificate {
    constructor(public cert:Buffer,
                /** Denoting if the Certificate is anonymous or carrying its owner's identity. */
                public privLevel?:PrivacyLevel) {
    }

    encode():Buffer {
        return this.cert;
    }
}

export class ECert extends Certificate {

    constructor(public cert:Buffer,
                public privateKey:any) {
        super(cert, PrivacyLevel.Nominal);
    }

}

export class TCert extends Certificate {
    constructor(public publicKey:any,
                public privateKey:any) {
        super(publicKey, PrivacyLevel.Anonymous);
    }
}

export interface TransactionRequest {
    getType():string;
}

export interface BuildRequest extends TransactionRequest {
}

export interface DeployRequest extends TransactionRequest {
}

export interface InvokeRequest extends TransactionRequest {
    chainCodeId:string;
    fcn:string;
    args:string[];
    confidential?:boolean;
    invoker?:Buffer;
}

export interface QueryRequest extends TransactionRequest {
}

export interface Transaction {
    getType():string;
    setCert(cert:Buffer):void;
    setSignature(sig:Buffer):void;
    setConfidentialityLevel(value:number): void;
    getConfidentialityLevel(): number;
    setConfidentialityProtocolVersion(version:string):void;
    setNonce(nonce:Buffer):void;
    setToValidators(Buffer):void;
    getChaincodeID():{buffer: Buffer};
    setChaincodeID(buffer:Buffer):void;
    getMetadata():{buffer: Buffer};
    setMetadata(buffer:Buffer):void;
    getPayload():{buffer: Buffer};
    setPayload(buffer:Buffer):void;
    toBuffer():Buffer;
}

export interface ErrorCallback { (err:Error):void
}

export interface GetValueCallback { (err:Error, value?:string):void
}

export class Chain {

    // Name of the chain is only meaningful to the client (currently)
    private name:string;

    // The peers on this chain to which the client can connect
    private peers:Peer[] = [];

    // Security enabled flag
    private securityEnabled:boolean = true;

    // A member cache associated with this chain
    // TODO: Make an LRU to limit size of member cache
    private members:{[name:string]:Member} = {};

    // The number of tcerts to get in each batch
    private tcertBatchSize:number = 200;

    // The registrar (if any) that registers & enrolls new members/users
    private registrar:Member;

    // The member services used for this chain
    private memberServices:MemberServices;

    // The key-val store used for this chain
    private keyValStore:KeyValStore;

    // The crypto primitives object
    cryptoPrimitives:crypto.Crypto;

    constructor(name:string) {
        this.name = name;
    }

    /**
     * Get the chain name.
     * @returns The name of the chain.
     */
    getName():string {
        return this.name;
    }

    /**
     * Add a peer given an endpoint specification.
     * @param endpoint The endpoint of the form: { url: "grpcs://host:port", tls: { .... } }
     * @returns {Peer} Returns a new peer.
     */
    addPeer(url:string):Peer {
        let peer = new Peer(url, this);
        this.peers.push(peer);
        return peer;
    };

    /**
     * Get the peers for this chain.
     */
    getPeers():Peer[] {
        return this.peers;
    }

    /**
     * Get the member whose credentials are used to register and enroll other users, or undefined if not set.
     * @param {Member} The member whose credentials are used to perform registration, or undefined if not set.
     */
    getRegistrar():Member {
        return this.registrar;
    }

    /**
     * Set the member whose credentials are used to register and enroll other users.
     * @param {Member} registrar The member whose credentials are used to perform registration.
     */
    setRegistrar(registrar:Member):void {
        this.registrar = registrar;
    }

    /**
     * Set the member services URL
     * @param {string} url Member services URL of the form: "grpc://host:port" or "grpcs://host:port"
     */
    setMemberServicesUrl(url:string):void {
        this.setMemberServices(newMemberServices(url));
    }

    /**
     * Get the member service associated this chain.
     * @returns {MemberService} Return the current member service, or undefined if not set.
     */
    getMemberServices():MemberServices {
        return this.memberServices;
    };

    /**
     * Set the member service associated this chain.  This allows the default implementation of member service to be overridden.
     */
    setMemberServices(memberServices:MemberServices):void {
        this.memberServices = memberServices;
        if (memberServices instanceof MemberServicesImpl) {
           this.cryptoPrimitives = (<MemberServicesImpl>memberServices).getCrypto();
        }
    };

    /**
     * Determine if security is enabled.
     */
    isSecurityEnabled():boolean {
        return this.memberServices !== undefined;
    }

    /**
     * Get the key val store implementation (if any) that is currently associated with this chain.
     * @returns {KeyValStore} Return the current KeyValStore associated with this chain, or undefined if not set.
     */
    getKeyValStore():KeyValStore {
        return this.keyValStore;
    }

    /**
     * Set the key value store implementation.
     */
    setKeyValStore(keyValStore:KeyValStore):void {
        this.keyValStore = keyValStore;
    }

    /**
     * Get the tcert batch size.
     */
    getTCertBatchSize():number {
        return this.tcertBatchSize;
    }

    /**
     * Set the tcert batch size.
     */
    setTCertBatchSize(batchSize:number) {
        this.tcertBatchSize = batchSize;
    }

    /**
     * Get the user member named 'name'.
     * @param cb Callback of form "function(err,Member)"
     */
    getMember(name:string, cb:GetMemberCallback):void {
        let self = this;
        cb = cb || nullCB;
        if (!self.keyValStore) return cb(Error("No key value store was found.  You must first call Chain.configureKeyValStore or Chain.setKeyValStore"));
        if (!self.memberServices) return cb(Error("No member services was found.  You must first call Chain.configureMemberServices or Chain.setMemberServices"));
        self.getMemberHelper(name, function (err, member) {
            if (err) return cb(err);
            cb(null, member);
        });
    }

    getUser(name:string, cb:GetMemberCallback):void {
        return this.getMember(name, cb);
    }

    // Try to get the member from cache.
    // If not found, create a new one, restore the state if found, and then store in cache.
    private getMemberHelper(name:string, cb:GetMemberCallback) {
        let self = this;
        // Try to get the member state from the cache
        let member = self.members[name];
        if (member) return cb(null, member);
        // Create the member and try to restore it's state from the key value store (if found).
        member = new Member(name, self);
        member.restoreState(function (err) {
            if (err) return cb(err);
            cb(null, member);
        });
    }

    /**
     * Send a transaction to a peer.
     * @param tx A transaction
     * @param eventEmitter An event emitter
     */
    sendTransaction(tx:Transaction, eventEmitter:events.EventEmitter) {
        let self = this;
        if (self.peers.length === 0) {
            return eventEmitter.emit('error', new Error(util.format("chain %s has no peers", self.getName())));
        }
        // Always send to 1st peer for now.  TODO: failover
        let peer = self.peers[0];
        peer.sendTransaction(tx, eventEmitter);
    }

}

export class Member {

    private chain:Chain;
    private name:string;
    private roles:string[];
    private account:string;
    private affiliation:string;
    private enrollmentSecret:string;
    private enrollment:any;
    private memberServices:MemberServices;
    private keyValStore:KeyValStore;
    private keyValStoreName:string;
    private tcerts:any[] = [];
    private tcertBatchSize:number;

    /**
     * Constructor for a member.
     * @param cfg {string | RegistrationRequest} The member name or registration request.
     * @returns {Member} A member who is neither registered nor enrolled.
     */
    constructor(cfg:any, chain:Chain) {
        if (util.isString(cfg)) {
            this.name = cfg;
        } else if (util.isObject(cfg)) {
            let req = cfg;
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
    getName():string {
        return this.name;
    }

    /**
     * Get the chain.
     * @returns {Chain} The chain.
     */
    getChain():Chain {
        return this.chain;
    };

    /**
     * Get the roles.
     * @returns {string[]} The roles.
     */
    getRoles():string[] {
        return this.roles;
    };

    /**
     * Set the roles.
     * @param roles {string[]} The roles.
     */
    setRoles(roles:string[]):void {
        this.roles = roles;
    };

    /**
     * Get the account.
     * @returns {string} The account.
     */
    getAccount():string {
        return this.account;
    };

    /**
     * Set the account.
     * @param account The account.
     */
    setAccount(account:string):void {
        this.account = account;
    };

    /**
     * Get the affiliation.
     * @returns {string} The affiliation.
     */
    getAffiliation():string {
        return this.affiliation;
    };

    /**
     * Set the affiliation.
     * @param affiliation The affiliation.
     */
    setAffiliation(affiliation:string):void {
        this.affiliation = affiliation;
    };

    /**
     * Get the transaction certificate (tcert) batch size, which is the number of tcerts retrieved
     * from member services each time (i.e. in a single batch).
     * @returns The tcert batch size.
     */
    getTCertBatchSize():number {
        if (this.tcertBatchSize === undefined) {
            return this.chain.getTCertBatchSize();
        } else {
            return this.tcertBatchSize;
        }
    }

    /**
     * Set the transaction certificate (tcert) batch size.
     * @param batchSize
     */
    setTCertBatchSize(batchSize:number):void {
        this.tcertBatchSize = batchSize;
    }

    /**
     * Get the enrollment info.
     * @returns {Enrollment} The enrollment.
     */
    getEnrollment():any {
        return this.enrollment;
    };

    /**
     * Determine if this name has been registered.
     * @returns {boolean} True if registered; otherwise, false.
     */
    isRegistered():boolean {
        return this.enrollmentSecret !== undefined;
    }

    /**
     * Determine if this name has been enrolled.
     * @returns {boolean} True if enrolled; otherwise, false.
     */
    isEnrolled():boolean {
        return this.enrollment !== undefined;
    }

    /**
     * Register the member.
     * @param cb Callback of the form: {function(err,enrollmentSecret)}
     */
    register(registrationRequest:RegistrationRequest, cb:RegisterCallback):void {
        let self = this;
        cb = cb || nullCB;
        let enrollmentSecret = this.enrollmentSecret;
        if (enrollmentSecret) {
            debug("previously registered, enrollmentSecret=%s", enrollmentSecret);
            return cb(null, enrollmentSecret);
        }
        self.memberServices.register(registrationRequest, function (err, enrollmentSecret) {
            debug("memberServices.register err=%s, secret=%s", err, enrollmentSecret);
            if (err) return cb(err);
            self.enrollmentSecret = enrollmentSecret;
            self.saveState(function (err) {
                if (err) return cb(err);
                cb(null, enrollmentSecret);
            });
        });
    }

    /**
     * Enroll the member and return the enrollment results.
     * @param enrollmentSecret The password or enrollment secret as returned by register.
     * @param cb Callback to report an error if it occurs
     */
    enroll(enrollmentSecret:string, cb:EnrollCallback):void {
        let self = this;
        cb = cb || nullCB;
        let enrollment = self.enrollment;
        if (enrollment) {
            debug("Previously enrolled, [enrollment=%j]", enrollment);
            return cb(null,enrollment);
        }
        let req = {enrollmentID: self.getName(), enrollmentSecret: enrollmentSecret};
        debug("Enrolling [req=%j]", req);
        self.memberServices.enroll(req, function (err:Error, enrollment:Enrollment) {
            debug("[memberServices.enroll] err=%s, enrollment=%j", err, enrollment);
            if (err) return cb(err);
            self.enrollment = enrollment;
            // Generate queryStateKey
            self.enrollment.queryStateKey = self.chain.cryptoPrimitives.generateNonce();

            // Save state
            self.saveState(function (err) {
                if (err) return cb(err);

                // Unmarshall chain key
                // TODO: during restore, unmarshall enrollment.chainKey
                debug("[memberServices.enroll] Unmarshalling chainKey");
                var ecdsaChainKey = self.chain.cryptoPrimitives.ecdsaPEMToPublicKey(self.enrollment.chainKey);
                self.enrollment.enrollChainKey = ecdsaChainKey;

                cb(null, enrollment);
            });
        });
    }

    /**
     * Perform both registration and enrollment.
     * @param cb Callback of the form: {function(err,{key,cert,chainKey})}
     */
    registerAndEnroll(registrationRequest:RegistrationRequest, cb:ErrorCallback):void {
        let self = this;
        cb = cb || nullCB;
        let enrollment = self.enrollment;
        if (enrollment) {
            debug("previously enrolled, enrollment=%j", enrollment);
            return cb(null);
        }

        self.register(registrationRequest, function (err, enrollmentSecret) {
            if (err) return cb(err);

            self.enroll(enrollmentSecret, function (err, enrollment) {
                if (err) return cb(err);

                cb(null);
            });

        });
    }

    /**
     * Issue a deploy request on behalf of this member.
     * @param deployRequest {Object}
     * @returns {TransactionContext} Emits 'submitted', 'complete', and 'error' events.
     */
    deploy(deployRequest:DeployRequest):TransactionContext {
        let tx = this.newTransactionContext();
        tx.deploy(deployRequest);
        return tx;
    }

    /**
     * Issue a invoke request on behalf of this member.
     * @param invokeRequest {Object}
     * @returns {TransactionContext} Emits 'submitted', 'complete', and 'error' events.
     */
    invoke(invokeRequest:InvokeRequest):TransactionContext {
        let tx = this.newTransactionContext();
        tx.invoke(invokeRequest);
        return tx;
    }

    /**
     * Issue a query request on behalf of this member.
     * @param queryRequest {Object}
     * @returns {TransactionContext} Emits 'submitted', 'complete', and 'error' events.
     */
    query(queryRequest:QueryRequest):TransactionContext {
        var tx = this.newTransactionContext();
        tx.query(queryRequest);
        return tx;
    }

    /**
     * Create a transaction context with which to issue build, deploy, invoke, or query transactions.
     * Only call this if you want to use the same tcert for multiple transactions.
     * @param {Object} tcert A transaction certificate from member services.  This is optional.
     * @returns {TransactionContext} Emits 'submitted', 'complete', and 'error' events.
     */
    newTransactionContext(tcert?:TCert):TransactionContext {
        return new TransactionContext(this, tcert);
    }

    getUserCert(cb:GetTCertCallback):void {
        this.getNextTCert(cb);
    }

    /**
     * Get the next available transaction certificate.
     * @param cb
     * @returns
     */
    getNextTCert(cb:GetTCertCallback):void {
        let self = this;
        if (!self.isEnrolled()) {
            return cb(Error(util.format("user '%s' is not enrolled",self.getName())));
        }
        if (self.tcerts.length > 0) {
            return cb(null, self.tcerts.shift());
        }
        let req = {
            name: self.getName(),
            enrollment: self.enrollment,
            num: self.getTCertBatchSize()
        };
        self.memberServices.getTCerts(req, function (err, tcerts) {
            if (err) return cb(err);
            self.tcerts = tcerts;
            return cb(null, self.tcerts.shift());
        });
    }

    /**
     * Save the state of this member to the key value store.
     * @param cb Callback of the form: {function(err}
     */
    saveState(cb:ErrorCallback):void {
        let self = this;
        self.keyValStore.setValue(self.keyValStoreName, self.toString(), cb);
    }

    /**
     * Restore the state of this member from the key value store (if found).  If not found, do nothing.
     * @param cb Callback of the form: function(err}
     */
    restoreState(cb:ErrorCallback):void {
        var self = this;
        self.keyValStore.getValue(self.keyValStoreName, function (err, memberStr) {
            if (err) return cb(err);
            // debug("restoreState: name=%s, memberStr=%s", self.getName(), memberStr);
            if (memberStr) {
                // The member was found in the key value store, so restore the state.
                self.fromString(memberStr);
            }
            cb(null);
        });
    }

    /**
     * Get the current state of this member as a string
     * @return {string} The state of this member as a string
     */
    fromString(str:string):void {
        let state = JSON.parse(str);
        if (state.name !== this.getName()) throw Error("name mismatch: '" + state.name + "' does not equal '" + this.getName() + "'");
        this.name = state.name;
        this.roles = state.roles;
        this.account = state.account;
        this.affiliation = state.affiliation;
        this.enrollmentSecret = state.enrollmentSecret;
        this.enrollment = state.enrollment;
    }

    /**
     * Save the current state of this member as a string
     * @return {string} The state of this member as a string
     */
    toString():string {
        let self = this;
        let state = {
            name: self.name,
            roles: self.roles,
            account: self.account,
            affiliation: self.affiliation,
            enrollmentSecret: self.enrollmentSecret,
            enrollment: self.enrollment
        };
        return JSON.stringify(state);
    }

}

/**
 * A transaction context emits events 'submitted', 'complete', and 'error'.
 * Each transaction context uses exactly one tcert.
 */
export class TransactionContext extends events.EventEmitter {

    private member:Member;
    private chain:Chain;
    private memberServices:MemberServices;
    private nonce: any;
    private binding: any;
    private tcert:TCert;

    constructor(member:Member, tcert:TCert) {
        super();
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
    getMember():Member {
        return this.member;
    }

    /**
     * Get the chain with which this transaction context is associated.
     * @returns The chain
     */
    getChain():Chain {
        return this.chain;
    };

    /**
     * Get the member services, or undefined if security is not enabled.
     * @returns The member services
     */
    getMemberServices():MemberServices {
        return this.memberServices;
    };

    /**
     * Issue a deploy transaction.
     * @param deployRequest {Object} A deploy request of the form: { chaincodeID, payload, metadata, uuid, timestamp, confidentiality: { level, version, nonce }
   */
    deploy(deployRequest:DeployRequest):TransactionContext {
        let self = this;
        self.getMyTCert(function (err) {
            if (err) {
                debug('Failed getting a new TCert [%s]', err);
                self.emit('error', err);

                return self
            }

            return self.execute(self.newBuildOrDeployTransaction(deployRequest, false));
        });
        return self;
    }

    /**
     * Issue an invoke transaction.
     * @param invokeRequest {Object} An invoke request of the form: XXX
     */
    invoke(invokeRequest:InvokeRequest):TransactionContext {
        let self = this;
        self.getMyTCert(function (err, tcert) {
            if (err) {
                debug('Failed getting a new TCert [%s]', err);
                self.emit('error', err);

                return self;
            }

            return self.execute(self.newInvokeOrQueryTransaction(invokeRequest, true));
        });
        return self;
    }

    /**
     * Issue an query transaction.
     * @param queryRequest {Object} A query request of the form: XXX
     */
    query(queryRequest:QueryRequest):TransactionContext {
        let self = this;

        self.getMyTCert(function (err, tcert) {
            if (err) {
                debug('Failed getting a new TCert [%s]', err);
                self.emit('error', err);

                return self;
            }

            return self.execute(self.newInvokeOrQueryTransaction(queryRequest, false));
        });

        return self;
    }

    /**
     * Execute a transaction
     * @param tx {Transaction} The transaction.
     */
    private execute(tx:Transaction):TransactionContext {
        debug('Executing transaction [%j]', tx);

        let self = this;
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
                let txBytes = tx.toBuffer();
                let derSignature = self.chain.cryptoPrimitives.ecdsaSign(tcert.privateKey.getPrivate('hex'), txBytes).toDER();
                // debug('signature: ', derSignature);
                tx.setSignature(new Buffer(derSignature));

                debug('Send transaction...');
                debug('Confidentiality: ', tx.getConfidentialityLevel());

                if (tx.getConfidentialityLevel() == _fabricProto.ConfidentialityLevel.CONFIDENTIAL &&
                        tx.getType() == _fabricProto.Transaction.Type.CHAINCODE_QUERY) {

                    var emitter = new events.EventEmitter();
                    emitter.on("complete", function (results) {
                        debug("Encrypted: [%j]", results);

                        let res = self.decryptResult(results);

                        debug("Decrypted: [%j]", res);

                        self.emit("complete", res);
                    });
                    emitter.on("error", function (results) {
                        self.emit("error", results);
                    });

                    self.getChain().sendTransaction(tx, emitter);
                } else {
                    self.getChain().sendTransaction(tx, self);
                }
            } else {
                debug('Missing TCert...');
                return self.emit('error', 'Missing TCert.')
            }

        });
        return self;
    }

    private getMyTCert(cb:GetTCertCallback): void {
        let self = this;
        if (!self.getChain().isSecurityEnabled() || self.tcert) {
            debug('[TransactionContext] TCert already cached.');
            return cb(null, self.tcert);
        }
        debug('[TransactionContext] No TCert cached. Retrieving one.');
        this.member.getNextTCert(function (err, tcert) {
            if (err) return cb(err);
            self.tcert = tcert;
            return cb(null, tcert);
        });
    }

    private processConfidentiality(transaction:Transaction) {
        // is confidentiality required?
        if (transaction.getConfidentialityLevel() != _fabricProto.ConfidentialityLevel.CONFIDENTIAL) {
            // No confidentiality is required
            return
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
        } else if (transaction.getType() == _fabricProto.Transaction.Type.CHAINCODE_INVOKE ) {
            // The request is for an execute
            // Empty state key
            stateKey = new Buffer([]);
        } else {
            // The request is for a query
            debug('Generate state key...');
            stateKey = new Buffer(self.chain.cryptoPrimitives.hmacAESTruncated(
                self.member.getEnrollment().queryStateKey,
                [CONFIDENTIALITY_1_2_STATE_KD_C6].concat(self.nonce)
            ));
        }

        // Prepare ciphertexts

        // Encrypts message to validators using self.enrollChainKey
        var chainCodeValidatorMessage1_2 = new asn1Builder.Ber.Writer();
        chainCodeValidatorMessage1_2.startSequence();
        chainCodeValidatorMessage1_2.writeBuffer(privBytes, 4);
        if (stateKey.length != 0) {
            debug('STATE KEY %j', stateKey);
            chainCodeValidatorMessage1_2.writeBuffer(stateKey, 4);
        } else {
            chainCodeValidatorMessage1_2.writeByte(4);
            chainCodeValidatorMessage1_2.writeLength(0);
        }
        chainCodeValidatorMessage1_2.endSequence();
        debug(chainCodeValidatorMessage1_2.buffer);

        debug('Using chain key [%j]', self.member.getEnrollment().chainKey);
        var ecdsaChainKey = self.chain.cryptoPrimitives.ecdsaPEMToPublicKey(
            self.member.getEnrollment().chainKey
        );

        let encMsgToValidators = self.chain.cryptoPrimitives.eciesEncryptECDSA(
            ecdsaChainKey,
            chainCodeValidatorMessage1_2.buffer
        );
        transaction.setToValidators(encMsgToValidators);

        // Encrypts chaincodeID using txKey
        // debug('CHAINCODE ID %j', transaction.chaincodeID);

        let encryptedChaincodeID = self.chain.cryptoPrimitives.eciesEncrypt(
            txKey.pubKeyObj,
            transaction.getChaincodeID().buffer
        );
        transaction.setChaincodeID(encryptedChaincodeID);

        // Encrypts payload using txKey
        // debug('PAYLOAD ID %j', transaction.payload);
        let encryptedPayload = self.chain.cryptoPrimitives.eciesEncrypt(
            txKey.pubKeyObj,
            transaction.getPayload().buffer
        );
        transaction.setPayload(encryptedPayload);

        // Encrypt metadata using txKey
        if (transaction.getMetadata() != null && transaction.getMetadata().buffer != null) {
            debug('Metadata [%j]', transaction.getMetadata().buffer);
            let encryptedMetadata = self.chain.cryptoPrimitives.eciesEncrypt(
                txKey.pubKeyObj,
                transaction.getMetadata().buffer
            );
            transaction.setMetadata(encryptedMetadata);
        }
    }

    private decryptResult(ct:Buffer) {
        let key = new Buffer(
            this.chain.cryptoPrimitives.hmacAESTruncated(
                this.member.getEnrollment().queryStateKey,
                [CONFIDENTIALITY_1_2_STATE_KD_C6].concat(this.nonce))
        );

        debug('Decrypt Result [%s]', ct.toString('hex'));
        return this.chain.cryptoPrimitives.aes256GCMDecrypt(key, ct);
    }

    /**
     * Create a deploy transaction.
     * @param request {Object} A BuildRequest or DeployRequest
     */
    private newBuildOrDeployTransaction(request:any, isBuildRequest:boolean):Transaction {
        let self = this;
        let tx = new _fabricProto.Transaction();
        if (isBuildRequest) {
            tx.setType(_fabricProto.Transaction.Type.CHAINCODE_BUILD);
        } else {
            tx.setType(_fabricProto.Transaction.Type.CHAINCODE_DEPLOY);
        }
        // Set the chaincodeID
        let chaincodeID = new _chaincodeProto.ChaincodeID();
        chaincodeID.setName(request.name);
        debug("newBuildOrDeployTransaction: chaincodeID: " + JSON.stringify(chaincodeID));
        tx.setChaincodeID(chaincodeID.toBuffer());

        // Construct the ChaincodeSpec
        let chaincodeSpec = new _chaincodeProto.ChaincodeSpec();
        // Set Type -- GOLANG is the only chaincode language supported at this time
        chaincodeSpec.setType(_chaincodeProto.ChaincodeSpec.Type.GOLANG);
        // Set chaincodeID
        chaincodeSpec.setChaincodeID(chaincodeID);
        // Set ctorMsg
        let chaincodeInput = new _chaincodeProto.ChaincodeInput();
        chaincodeInput.setFunction(request.function);
        chaincodeInput.setArgs(request.arguments);
        chaincodeSpec.setCtorMsg(chaincodeInput);

        // Construct the ChaincodeDeploymentSpec (i.e. the payload)
        let chaincodeDeploymentSpec = new _chaincodeProto.ChaincodeDeploymentSpec();
        chaincodeDeploymentSpec.setChaincodeSpec(chaincodeSpec);
        tx.setPayload(chaincodeDeploymentSpec.toBuffer());

        // Set the transaction UUID
        tx.setUuid(generateUUID());

        // Set the transaction timestamp
        tx.setTimestamp(generateTimestamp());

        // Set confidentiality level
        if ('confidential' in request && request.confidential) {
            debug('Set confidentiality on');
            tx.setConfidentialityLevel(_fabricProto.ConfidentialityLevel.CONFIDENTIAL)
        } else {
            debug('Set confidentiality on');
            tx.setConfidentialityLevel(_fabricProto.ConfidentialityLevel.PUBLIC)
        }

        if ('metadata' in request && request.metadata) {
            tx.setMetadata(request.metadata)
        }

        return tx;
    }

    /**
     * Create an invoke or query transaction.
     * @param request {Object} A build or deploy request of the form: { chaincodeID, payload, metadata, uuid, timestamp, confidentiality: { level, version, nonce }
 */
    private newInvokeOrQueryTransaction(request:any, isInvokeRequest:boolean):Transaction {
        let self = this;

        // Create a deploy transaction
        let tx = new _fabricProto.Transaction();
        if (isInvokeRequest) {
            tx.setType(_fabricProto.Transaction.Type.CHAINCODE_INVOKE);
        } else {
            tx.setType(_fabricProto.Transaction.Type.CHAINCODE_QUERY);
        }

        // Set the chaincodeID
        let chaincodeID = new _chaincodeProto.ChaincodeID();
        chaincodeID.setName(request.name);
        debug("newInvokeOrQueryTransaction: chaincodeID: " + JSON.stringify(chaincodeID));
        tx.setChaincodeID(chaincodeID.toBuffer());

        // Construct the ChaincodeSpec
        let chaincodeSpec = new _chaincodeProto.ChaincodeSpec();
        // Set Type -- GOLANG is the only chaincode language supported at this time
        chaincodeSpec.setType(_chaincodeProto.ChaincodeSpec.Type.GOLANG);
        // Set chaincodeID
        chaincodeSpec.setChaincodeID(chaincodeID);
        // Set ctorMsg
        let chaincodeInput = new _chaincodeProto.ChaincodeInput();
        chaincodeInput.setFunction(request.function);
        chaincodeInput.setArgs(request.arguments);
        chaincodeSpec.setCtorMsg(chaincodeInput);
        // Construct the ChaincodeInvocationSpec (i.e. the payload)
        let chaincodeInvocationSpec = new _chaincodeProto.ChaincodeInvocationSpec();
        chaincodeInvocationSpec.setChaincodeSpec(chaincodeSpec);
        tx.setPayload(chaincodeInvocationSpec.toBuffer());

        // Set the transaction UUID
        tx.setUuid(generateUUID());

        // Set the transaction timestamp
        tx.setTimestamp(generateTimestamp());

        // Set confidentiality level
        if ('confidential' in request && request.confidential) {
            debug('Set confidentiality on');
            tx.setConfidentialityLevel(_fabricProto.ConfidentialityLevel.CONFIDENTIAL)
        } else {
            debug('Set confidentiality on');
            tx.setConfidentialityLevel(_fabricProto.ConfidentialityLevel.PUBLIC)
        }

        if ('metadata' in request && request.metadata) {
            tx.setMetadata(request.metadata)
        }

        if ('invoker' in request && request.invoker != null) {

            if ('appCert' in request.invoker.appCert != null) {
                // cert based

                let certRaw = new Buffer(self.tcert.publicKey);
                // debug('========== Invoker Cert [%s]', certRaw.toString('hex'));
                let nonceRaw = new Buffer(self.nonce);

                let bindingMsg = Buffer.concat([certRaw, nonceRaw]);
                // debug('========== Binding Msg [%s]', bindingMsg.toString('hex'));
                this.binding = new Buffer(self.chain.cryptoPrimitives.hash(bindingMsg), 'hex');

                // debug('========== Binding [%s]', this.binding.toString('hex'));

                let ctor = chaincodeSpec.getCtorMsg().toBuffer();
                // debug('========== Ctor [%s]', ctor.toString('hex'));

                let txmsg = Buffer.concat([ctor, this.binding]);
                // debug('========== Pyaload||binding [%s]', txmsg.toString('hex'));
                let mdsig = self.chain.cryptoPrimitives.ecdsaSign(request.invoker.appCert.privateKey.getPrivate('hex'), txmsg);
                let sigma = new Buffer(mdsig.toDER());
                // debug('========== Sigma [%s]', sigma.toString('hex'));

                tx.setMetadata(sigma)
            }

        }

        return tx;
    }

}  // end TransactionContext

export class Peer {

    private url:string;
    private chain:Chain;
    private ep:Endpoint;
    private peerClient:any;

    /**
     * Constructor for a peer given the endpoint config for the peer.
     * @param {string} url The URL of
     * @param {Chain} The chain of which this peer is a member.
     * @returns {Peer} The new peer.
     */
    constructor(url:string, chain:Chain) {
        this.url = url;
        this.chain = chain;
        this.ep = new Endpoint(url);
        this.peerClient = new _fabricProto.Peer(this.ep.addr, this.ep.creds);
    }

    /**
     * Get the chain of which this peer is a member.
     * @returns {Chain} The chain of which this peer is a member.
     */
    getChain():Chain {
        return this.chain;
    }

    /**
     * Get the URL of the peer.
     * @returns {string} Get the URL associated with the peer.
     */
    getUrl():string {
        return this.url;
    }

    /**
     * Send a transaction to this peer.
     * @param tx A transaction
     * @param eventEmitter The event emitter
     */
    sendTransaction = function (tx:Transaction, eventEmitter:events.EventEmitter) {
        var self = this;

        // debug("peer.sendTransaction: sending %j", tx);

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
                let txType = tx.getType();
                switch (txType) {
                    case _fabricProto.Transaction.Type.CHAINCODE_DEPLOY: // async
                        if (response.status === "SUCCESS") {
                            // Deploy transaction has been submitted
                            eventEmitter.emit('submitted', response.msg);
                        } else {
                            // Deploy completed with status "FAILURE" or "UNDEFINED"
                            eventEmitter.emit('error', response.msg);
                        }
                        break;
                    case _fabricProto.Transaction.Type.CHAINCODE_INVOKE: // async
                        if (response.status === "SUCCESS") {
                            // Invoke transaction has been submitted
                            eventEmitter.emit('submitted', response.msg);
                        } else {
                            // Invoke completed with status "FAILURE" or "UNDEFINED"
                            eventEmitter.emit('error', response.msg);
                        }
                        break;
                    case _fabricProto.Transaction.Type.CHAINCODE_QUERY: // sync
                        if (response.status === "SUCCESS") {
                            // Query transaction has been completed
                            eventEmitter.emit('complete', response.msg);
                        } else {
                            // Query completed with status "FAILURE" or "UNDEFINED"
                            eventEmitter.emit('error', response.msg);
                        }
                        break;
                    default: // not implemented
                        eventEmitter.emit('error', new Error("processTransaction for this transaction type is not yet implemented!"));
                }
            }
        );
    };

    /**
     * Remove the peer from the chain.
     */
    remove():void {
        throw Error("TODO: implement");
    }

} // end Peer

/**
 * An endpoint currently takes only URL (currently).
 * @param url
 */
class Endpoint {

    addr:string;
    creds:Buffer;

    constructor(url:string) {
        let purl = parseUrl(url);
        let protocol = purl.protocol.toLowerCase();
        if (protocol === 'grpc') {
            this.addr = purl.host;
            this.creds = grpc.credentials.createInsecure();
        } else if (protocol === 'grpcs') {
            this.addr = purl.host;
            this.creds = grpc.credentials.createSsl();
        } else {
            throw Error("invalid protocol: " + protocol);
        }
    }
}

class MemberServicesImpl {

    private ecaaClient:any;
    private ecapClient:any;
    private tcapClient:any;
    private tlscapClient:any;
    private cryptoPrimitives:crypto.Crypto;

    /**
     * MemberServicesImpl constructor
     * @param config The config information required by this member services implementation.
     * @returns {MemberServices} A MemberServices object.
     */
    constructor(url:string) {
        let ep = new Endpoint(url);
        this.ecaaClient = new _caProto.ECAA(ep.addr, ep.creds);
        this.ecapClient = new _caProto.ECAP(ep.addr, ep.creds);
        this.tcapClient = new _caProto.TCAP(ep.addr, ep.creds);
        this.tlscapClient = new _caProto.TLSCAP(ep.addr, ep.creds);
        this.cryptoPrimitives = new crypto.Crypto(DEFAULT_HASH_ALGORITHM, DEFAULT_SECURITY_LEVEL);
    }

    /**
     * Get the security level
     * @returns The security level
     */
    getSecurityLevel():number {
        return this.cryptoPrimitives.getSecurityLevel();
    }

    /**
     * Set the security level
     * @params securityLevel The security level
     */
    setSecurityLevel(securityLevel:number):void {
        this.cryptoPrimitives.setSecurityLevel(securityLevel);
    }

    /**
     * Get the hash algorithm
     * @returns {string} The hash algorithm
     */
    getHashAlgorithm():string {
        return this.cryptoPrimitives.getHashAlgorithm();
    }

    /**
     * Set the hash algorithm
     * @params hashAlgorithm The hash algorithm ('SHA2' or 'SHA3')
     */
    setHashAlgorithm(hashAlgorithm:string):void {
        this.cryptoPrimitives.setHashAlgorithm(hashAlgorithm);
    }

    getCrypto():crypto.Crypto {
        return this.cryptoPrimitives;
    }

    /**
     * Register the member and return an enrollment secret.
     * @param req Registration request with the following fields: name, role
     * @param cb Callback of the form: {function(err,enrollmentSecret)}
     */
    register(req:RegistrationRequest, cb:RegisterCallback):void {
        var self = this;
        debug("MemberServicesImpl.register %j", req);
        if (!req.enrollmentID) return cb(new Error("missing req.enrollmentID"));
        var protoReq = new _caProto.RegisterUserReq();
        protoReq.setId({id: req.enrollmentID});
        protoReq.setRole(rolesToMask(req.roles));
        protoReq.setAccount(req.account);
        protoReq.setAffiliation(req.affiliation);
        self.ecaaClient.registerUser(protoReq, function (err, token) {
            debug("register %j: err=%j, token=%s", protoReq, err, token);
            if (cb) return cb(err, token ? token.tok.toString() : null);
        });
    }

    /**
     * Enroll the member and return an opaque member object
     * @param req Enrollment request with the following fields: name, enrollmentSecret
     * @param cb Callback of the form: {function(err,{key,cert,chainKey})}
     */
    enroll(req:EnrollmentRequest, cb:EnrollCallback):void {
        let self = this;
        cb = cb || nullCB;

        debug("[MemberServicesImpl.enroll] [%j]", req);
        if (!req.enrollmentID) return cb(Error("req.enrollmentID is not set"));
        if (!req.enrollmentSecret) return cb(Error("req.enrollmentSecret is not set"));

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
        var timestamp = new _timeStampProto({seconds: Date.now() / 1000, nanos: 0});
        eCertCreateRequest.setTs(timestamp);
        eCertCreateRequest.setId({id: req.enrollmentID});
        eCertCreateRequest.setTok({tok: new Buffer(req.enrollmentSecret)});

        debug("[MemberServicesImpl.enroll] Generating request! %j", spki.getASN1Object().getEncodedHex());

        // public signing key (ecdsa)
        var signPubKey = new _caProto.PublicKey(
            {
                type: _caProto.CryptoType.ECDSA,
                key: new Buffer(spki.getASN1Object().getEncodedHex(), 'hex')
            });
        eCertCreateRequest.setSign(signPubKey);

        debug("[MemberServicesImpl.enroll] Adding signing key!");

        // public encryption key (ecdsa)
        var encPubKey = new _caProto.PublicKey(
            {
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
            let cipherText = eCertCreateResp.tok.tok;
            var decryptedTokBytes = self.cryptoPrimitives.eciesDecrypt(encryptionKeyPair.prvKeyObj, cipherText);

            //debug(decryptedTokBytes);
            // debug(decryptedTokBytes.toString());
            // debug('decryptedTokBytes [%s]', decryptedTokBytes.toString());
            eCertCreateRequest.setTok({tok: decryptedTokBytes});
            eCertCreateRequest.setSig(null);

            var buf = eCertCreateRequest.toBuffer();

            var signKey = self.cryptoPrimitives.ecdsaKeyFromPrivate(signingKeyPair.prvKeyObj.prvKeyHex, 'hex');
            //debug(new Buffer(sha3_384(buf),'hex'));
            var sig = self.cryptoPrimitives.ecdsaSign(signKey, buf);

            eCertCreateRequest.setSig(new _caProto.Signature(
                {
                    type: _caProto.CryptoType.ECDSA,
                    r: new Buffer(sig.r.toString()),
                    s: new Buffer(sig.s.toString())
                }
            ));
            self.ecapClient.createCertificatePair(eCertCreateRequest, function (err, eCertCreateResp) {
                if (err) return cb(err);
                debug('[MemberServicesImpl.enroll] eCertCreateResp : [%j]' + eCertCreateResp);

                let enrollment = {
                    key: signingKeyPair.prvKeyObj.prvKeyHex,
                    cert: eCertCreateResp.certs.sign.toString('hex'),
                    chainKey: eCertCreateResp.pkchain.toString('hex')
                };
                // debug('cert:\n\n',enrollment.cert)
                cb(null, enrollment);
            });
        });

    } // end enroll

    /**
     * Get an array of transaction certificates (tcerts).
     * @param {Object} req Request of the form: {name,enrollment,num} where
     * 'name' is the member name,
     * 'enrollment' is what was returned by enroll, and
     * 'num' is the number of transaction contexts to obtain.
     * @param {function(err,[Object])} cb The callback function which is called with an error as 1st arg and an array of tcerts as 2nd arg.
     */
    getTCerts(req:GetTCertsRequest, cb:GetTCertsCallback): void {
        let self = this;
        cb = cb || nullCB;

        let timestamp = new _timeStampProto({seconds: Date.now() / 1000, nanos: 0});

        // create the proto
        let tCertCreateSetReq = new _caProto.TCertCreateSetReq();
        tCertCreateSetReq.setTs(timestamp);
        tCertCreateSetReq.setId({id: req.name});
        tCertCreateSetReq.setNum(req.num);

        // serialize proto
        let buf = tCertCreateSetReq.toBuffer();

        // sign the transaction using enrollment key
        let signKey = self.cryptoPrimitives.ecdsaKeyFromPrivate(req.enrollment.key, 'hex');
        let sig = self.cryptoPrimitives.ecdsaSign(signKey, buf);

        tCertCreateSetReq.setSig(new _caProto.Signature(
            {
                type: _caProto.CryptoType.ECDSA,
                r: new Buffer(sig.r.toString()),
                s: new Buffer(sig.s.toString())
            }
        ));

        // send the request
        self.tcapClient.createCertificateSet(tCertCreateSetReq, function (err, resp) {
            if (err) return cb(err);
            // debug('tCertCreateSetResp:\n', resp);
            cb(null, self.processTCerts(req, resp));
        });
    }

    private processTCerts(req:GetTCertsRequest, resp:any):TCert[] {
        let self = this;

        //
        // Derive secret keys for TCerts
        //

        let enrollKey = req.enrollment.key;
        let tCertOwnerKDFKey = resp.certs.key;
        let tCerts = resp.certs.certs;

        let byte1 = new Buffer(1);
        byte1.writeUInt8(0x1, 0);
        let byte2 = new Buffer(1);
        byte2.writeUInt8(0x2, 0);

        let tCertOwnerEncryptKey = self.cryptoPrimitives.hmac(tCertOwnerKDFKey, byte1).slice(0, 32);
        let expansionKey = self.cryptoPrimitives.hmac(tCertOwnerKDFKey, byte2);

        let tCertBatch:TCert[] = [];

        // Loop through certs and extract private keys
        for (var i = 0; i < tCerts.length; i++) {
            var tCert = tCerts[i];
            let x509Certificate;
            try {
                x509Certificate = new crypto.X509Certificate(tCert.cert);
            } catch (ex) {
                debug('Error parsing certificate bytes: ', ex);
                continue
            }

            // debug("HERE2: got x509 cert");
            // extract the encrypted bytes from extension attribute
            let tCertIndexCT = x509Certificate.criticalExtension(crypto.TCertEncTCertIndex);
            // debug('tCertIndexCT: ',JSON.stringify(tCertIndexCT));
            let tCertIndex = self.cryptoPrimitives.aesCBCPKCS7Decrypt(tCertOwnerEncryptKey, tCertIndexCT);
            // debug('tCertIndex: ',JSON.stringify(tCertIndex));

            let expansionValue = self.cryptoPrimitives.hmac(expansionKey, tCertIndex);
            // debug('expansionValue: ',expansionValue);

            // compute the private key
            let one = new BN(1);
            let k = new BN(expansionValue);
            let n = self.cryptoPrimitives.ecdsaKeyFromPrivate(enrollKey, 'hex').ec.curve.n.sub(one);
            k = k.mod(n).add(one);

            let D = self.cryptoPrimitives.ecdsaKeyFromPrivate(enrollKey, 'hex').getPrivate().add(k);
            let pubHex = self.cryptoPrimitives.ecdsaKeyFromPrivate(enrollKey, 'hex').getPublic('hex');
            D = D.mod(self.cryptoPrimitives.ecdsaKeyFromPublic(pubHex, 'hex').ec.curve.n);

            // Put private and public key in returned tcert
            let tcert = new TCert(tCert.cert, self.cryptoPrimitives.ecdsaKeyFromPrivate(D, 'hex'));
            tCertBatch.push(tcert);
        }

        if (tCertBatch.length == 0) {
            throw Error('Failed fetching TCerts. No valid TCert received.')
        }

        return tCertBatch;

    } // end processTCerts

} // end MemberServicesImpl

function newMemberServices(url) {
    return new MemberServicesImpl(url);
}

/**
 * A local file-based key value store.
 * This implements the KeyValStore interface.
 */
class FileKeyValStore implements KeyValStore {

    private dir:string;   // root directory for the file store

    constructor(dir:string) {
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
    getValue(name:string, cb:GetValueCallback) {
        var path = this.dir + '/' + name;
        fs.readFile(path, 'utf8', function (err, data) {
            if (err) {
                if (err.code !== 'ENOENT') return cb(err);
                return cb(null, null);
            }
            return cb(null, data);
        });
    }

    /**
     * Set the value associated with name.
     * @param name
     * @param cb function(err)
     */
    setValue = function (name:string, value:string, cb:ErrorCallback) {
        var path = this.dir + '/' + name;
        fs.writeFile(path, value, cb);
    }

} // end FileKeyValStore

/**
 * generateUUID returns an RFC4122 compliant UUID.
 *    http://www.ietf.org/rfc/rfc4122.txt
 */
function generateUUID() {
    return uuid.v4();
};

/**
 * generateTimestamp returns the current time in the google/protobuf/timestamp.proto structure.
 */
function generateTimestamp() {
    return new _timeStampProto({seconds: Date.now() / 1000, nanos: 0});
}

function toKeyValStoreName(name:string):string {
    return "member." + name;
}

/**
 * Create and load peers for bluemix.
 */
function bluemixInit():boolean {
    var vcap = process.env['VCAP_SERVICES'];
    if (!vcap) return false; // not in bluemix
    // TODO: Take logic from marbles app
    return true;
}

function nullCB():void {
}

// Determine if an object is a string
function isString(obj:any):boolean {
    return (typeof obj === 'string' || obj instanceof String);
}

// Determine if 'obj' is an object (not an array, string, or other type)
function isObject(obj:any):boolean {
    return (!!obj) && (obj.constructor === Object);
}

function isFunction(fcn:any):boolean {
    return (typeof fcn === 'function');
}

function parseUrl(url:string):any {
    // TODO: find ambient definition for url
    var purl = urlParser.parse(url, true);
    var protocol = purl.protocol;
    if (endsWith(protocol, ":")) {
        purl.protocol = protocol.slice(0, -1);
    }
    return purl;
}

// Convert a list of role names to the role mask currently used by the peer
// See the mapping of role names to the mask values below
function rolesToMask(roles?:string[]):number {
    let mask:number = 0;
    if (roles) {
        for (let role in roles) {
            switch (role) {
                case 'fabric.user':
                    mask |= 1;
                    break;       // Client mask
                case 'fabric.peer':
                    mask |= 2;
                    break;       // Peer mask
                case 'fabric.validator':
                    mask |= 4;
                    break;  // Validator mask
                case 'fabric.auditor':
                    mask |= 8;
                    break;    // Auditor mask
            }
        }
    }
    if (mask === 0) mask = 1;  // Client
    return mask;
}

function endsWith(str:string, suffix:string) {
    return str.length >= suffix.length && str.substr(str.length - suffix.length) === suffix;
};

/**
 * Create a new chain.  If it already exists, throws an Error.
 * @param name {string} Name of the chain.  It can be any name and has value only for the client.
 * @returns
 */
export function newChain(name) {
    let chain = _chains[name];
    if (chain) throw Error(util.format("chain %s already exists", name));
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
export function getChain(chainName, create) {
    let chain = _chains[chainName];
    if (!chain && create) {
        chain = newChain(name);
    }
    return chain;
}

/**
 * Stop/cleanup everything pertaining to this module.
 */
export function stop() {
    // Shutdown each chain
    for (var chainName in _chains) {
        _chains[chainName].shutdown();
    }
}

export function newFileKeyValStore(dir:string):KeyValStore {
    return new FileKeyValStore(dir);
}
