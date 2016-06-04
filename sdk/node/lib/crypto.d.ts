export declare const TCertEncTCertIndex: string;
/**
 * The crypto class contains implementations of various crypto primitives.
 */
export declare class Crypto {
    private hashAlgorithm;
    private securityLevel;
    private curveName;
    private suite;
    private hashFunction;
    private hashFunctionKeyDerivation;
    private hashOutputSize;
    private ecdsaCurve;
    constructor(hashAlgorithm: string, securityLevel: number);
    /**
     * Get the security level
     * @returns The security level
     */
    getSecurityLevel(): number;
    /**
     * Set the security level
     * @params securityLevel The security level
     */
    setSecurityLevel(securityLevel: number): void;
    /**
     * Get the hash algorithm
     * @returns {string} The hash algorithm
     */
    getHashAlgorithm(): string;
    /**
     * Set the hash algorithm
     * @params hashAlgorithm The hash algorithm ('SHA2' or 'SHA3')
     */
    setHashAlgorithm(hashAlgorithm: string): void;
    generateNonce(): any;
    ecdsaKeyGen(): any;
    ecdsaKeyFromPrivate(key: any, encoding: any): any;
    ecdsaKeyFromPublic(key: any, encoding: any): any;
    ecdsaSign(key: Buffer, msg: Buffer): any;
    ecdsaPEMToPublicKey(chainKey: any): any;
    ecdsaPrivateKeyToASN1(prvKeyHex: string): Buffer;
    eciesKeyGen(): any;
    eciesEncryptECDSA(ecdsaRecipientPublicKey: any, msg: any): Buffer;
    eciesEncrypt(recipientPublicKey: any, msg: any): Buffer;
    eciesDecrypt(recipientPrivateKey: any, cipherText: any): any;
    aesKeyGen(): any;
    aesCFBDecryt(key: any, encryptedBytes: any): any;
    aesCBCPKCS7Decrypt(key: any, bytes: any): any;
    aes256GCMDecrypt(key: Buffer, ct: Buffer): any;
    hkdf(ikm: any, keyBitLength: any, salt: any, info: any): any[];
    hmac(key: any, bytes: any): any[];
    hmacAESTruncated(key: any, bytes: any): any[];
    hash(bytes: any): any;
    private checkSecurityLevel();
    private checkHashFunction();
    private initialize();
    /** HKDF with the specified hash function.
     * @param {bitArray} ikm The input keying material.
     * @param {Number} keyBitLength The output key length, in bits.
     * @param {String|bitArray} salt The salt for HKDF.
     * @param {String|bitArray} info The info for HKDF.
     * @param {Object} [Hash=sjcl.hash.sha256] The hash function to use.
     * @return {bitArray} derived key.
     */
    private hkdf2(ikm, keyBitLength, salt, info, Hash);
    private CBCDecrypt(key, bytes);
    private PKCS7UnPadding(bytes);
}
export declare class X509Certificate {
    private _cert;
    constructor(buffer: any);
    criticalExtension(oid: any): Buffer;
}
