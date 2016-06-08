import * as crypto from "./crypto";
import events = require('events');
export interface KeyValStore {
    getValue(name: string, cb: GetValueCallback): void;
    setValue(name: string, value: string, cb: ErrorCallback): any;
}
export interface MemberServices {
    getSecurityLevel(): number;
    setSecurityLevel(securityLevel: number): void;
    getHashAlgorithm(): string;
    setHashAlgorithm(hashAlgorithm: string): void;
    register(req: RegistrationRequest, registrar: Member, cb: RegisterCallback): void;
    enroll(req: EnrollmentRequest, cb: EnrollCallback): void;
    getTCertBatch(req: {
        name: string;
        enrollment: Enrollment;
        num: number;
    }, cb: GetTCertBatchCallback): void;
}
export interface RegistrationRequest {
    enrollmentID: string;
    roles?: string[];
    account: string;
    affiliation: string;
    registrar?: {
        roles: string[];
        delegateRoles?: string[];
    };
}
export interface EnrollmentRequest {
    enrollmentID: string;
    enrollmentSecret: string;
}
export interface GetMemberCallback {
    (err: Error, member?: Member): void;
}
export interface RegisterCallback {
    (err: Error, enrollmentPassword?: string): void;
}
export interface EnrollCallback {
    (err: Error, enrollment?: Enrollment): void;
}
export interface DeployTransactionCallback {
    (err: Error, deployTx?: Transaction): void;
}
export interface Enrollment {
    key: Buffer;
    cert: string;
    chainKey: string;
}
export interface GetTCertBatchRequest {
    name: string;
    enrollment: Enrollment;
    num: number;
}
export interface SubmittedTransactionResponse {
    uuid: string;
}
export interface GetTCertBatchCallback {
    (err: Error, tcerts?: TCert[]): void;
}
export interface GetTCertCallback {
    (err: Error, tcert?: TCert): void;
}
export interface GetTCertCallback {
    (err: Error, tcert?: TCert): void;
}
export declare enum PrivacyLevel {
    Nominal = 0,
    Anonymous = 1,
}
export declare class Certificate {
    cert: Buffer;
    privateKey: any;
    privLevel: PrivacyLevel;
    constructor(cert: Buffer, privateKey: any, privLevel?: PrivacyLevel);
    encode(): Buffer;
}
export declare class ECert extends Certificate {
    cert: Buffer;
    privateKey: any;
    constructor(cert: Buffer, privateKey: any);
}
export declare class TCert extends Certificate {
    publicKey: any;
    privateKey: any;
    constructor(publicKey: any, privateKey: any);
}
export interface TransactionRequest {
    chaincodeID: string;
    fcn: string;
    args: string[];
    confidential?: boolean;
    userCert?: Certificate;
    metadata?: Buffer;
}
export interface DeployRequest extends TransactionRequest {
    chaincodePath: string;
}
export interface InvokeOrQueryRequest extends TransactionRequest {
    attrs?: string[];
}
export interface QueryRequest extends InvokeOrQueryRequest {
}
export interface InvokeRequest extends InvokeOrQueryRequest {
}
export interface Transaction {
    getType(): string;
    setCert(cert: Buffer): void;
    setSignature(sig: Buffer): void;
    setConfidentialityLevel(value: number): void;
    getConfidentialityLevel(): number;
    setConfidentialityProtocolVersion(version: string): void;
    setNonce(nonce: Buffer): void;
    setToValidators(Buffer: any): void;
    getChaincodeID(): {
        buffer: Buffer;
    };
    setChaincodeID(buffer: Buffer): void;
    getMetadata(): {
        buffer: Buffer;
    };
    setMetadata(buffer: Buffer): void;
    getPayload(): {
        buffer: Buffer;
    };
    setPayload(buffer: Buffer): void;
    toBuffer(): Buffer;
}
export interface ErrorCallback {
    (err: Error): void;
}
export interface GetValueCallback {
    (err: Error, value?: string): void;
}
export declare class Chain {
    private name;
    private peers;
    private securityEnabled;
    private members;
    private tcertBatchSize;
    private registrar;
    private memberServices;
    private keyValStore;
    private devMode;
    private preFetchMode;
    cryptoPrimitives: crypto.Crypto;
    constructor(name: string);
    getName(): string;
    addPeer(url: string, pem?: string): Peer;
    getPeers(): Peer[];
    getRegistrar(): Member;
    setRegistrar(registrar: Member): void;
    setMemberServicesUrl(url: string, pem?: string): void;
    getMemberServices(): MemberServices;
    setMemberServices(memberServices: MemberServices): void;
    isSecurityEnabled(): boolean;
    isPreFetchMode(): boolean;
    setPreFetchMode(preFetchMode: boolean): void;
    isDevMode(): boolean;
    setDevMode(devMode: boolean): void;
    getKeyValStore(): KeyValStore;
    setKeyValStore(keyValStore: KeyValStore): void;
    getTCertBatchSize(): number;
    setTCertBatchSize(batchSize: number): void;
    getMember(name: string, cb: GetMemberCallback): void;
    getUser(name: string, cb: GetMemberCallback): void;
    private getMemberHelper(name, cb);
    register(registrationRequest: RegistrationRequest, cb: RegisterCallback): void;
    enroll(name: string, secret: string, cb: GetMemberCallback): void;
    registerAndEnroll(registrationRequest: RegistrationRequest, cb: GetMemberCallback): void;
    sendTransaction(tx: Transaction, eventEmitter: events.EventEmitter): boolean;
}
export declare class Member {
    private chain;
    private name;
    private roles;
    private account;
    private affiliation;
    private enrollmentSecret;
    private enrollment;
    private memberServices;
    private keyValStore;
    private keyValStoreName;
    private tcerts;
    private tcertBatchSize;
    private arrivalRate;
    private getTCertResponseTime;
    private getTCertWaiters;
    private gettingTCerts;
    constructor(cfg: any, chain: Chain);
    getName(): string;
    getChain(): Chain;
    getRoles(): string[];
    setRoles(roles: string[]): void;
    getAccount(): string;
    setAccount(account: string): void;
    getAffiliation(): string;
    setAffiliation(affiliation: string): void;
    getTCertBatchSize(): number;
    setTCertBatchSize(batchSize: number): void;
    getEnrollment(): any;
    isRegistered(): boolean;
    isEnrolled(): boolean;
    register(registrationRequest: RegistrationRequest, cb: RegisterCallback): void;
    enroll(enrollmentSecret: string, cb: EnrollCallback): void;
    registerAndEnroll(registrationRequest: RegistrationRequest, cb: ErrorCallback): void;
    deploy(deployRequest: DeployRequest): TransactionContext;
    invoke(invokeRequest: InvokeRequest): TransactionContext;
    query(queryRequest: QueryRequest): TransactionContext;
    newTransactionContext(tcert?: TCert): TransactionContext;
    getUserCert(cb: GetTCertCallback): void;
    getNextTCert(cb: GetTCertCallback): void;
    private shouldGetTCerts();
    private getTCerts();
    saveState(cb: ErrorCallback): void;
    restoreState(cb: ErrorCallback): void;
    fromString(str: string): void;
    toString(): string;
}
export declare class TransactionContext extends events.EventEmitter {
    private member;
    private chain;
    private memberServices;
    private nonce;
    private binding;
    private tcert;
    constructor(member: Member, tcert: TCert);
    getMember(): Member;
    getChain(): Chain;
    getMemberServices(): MemberServices;
    deploy(deployRequest: DeployRequest): TransactionContext;
    invoke(invokeRequest: InvokeRequest): TransactionContext;
    query(queryRequest: QueryRequest): TransactionContext;
    private execute(tx);
    private getMyTCert(cb);
    private processConfidentiality(transaction);
    private decryptResult(ct);
    private newBuildOrDeployTransaction(request, isBuildRequest, cb);
    private newInvokeOrQueryTransaction(request, isInvokeRequest);
}
export declare class Peer {
    private url;
    private chain;
    private ep;
    private peerClient;
    constructor(url: string, chain: Chain, pem: string);
    getChain(): Chain;
    getUrl(): string;
    sendTransaction: (tx: Transaction, eventEmitter: events.EventEmitter) => void;
    private waitToComplete(eventEmitter);
    remove(): void;
}
export declare function newChain(name: any): any;
export declare function getChain(chainName: any, create: any): any;
export declare function newFileKeyValStore(dir: string): KeyValStore;
