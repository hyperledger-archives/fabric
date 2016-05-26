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

// requires
var debug = require('debug')('crypto');
var aesjs = require('aes-js');
var crypto = require('crypto');
var elliptic = require('elliptic');
var EC = elliptic.ec;
var sha3_256 = require('js-sha3').sha3_256;
var sha3_384 = require('js-sha3').sha3_384;
var sjcl = require('sjcl');
var util = require("util");
var jsrsa = require('jsrsasign');
var KEYUTIL = jsrsa.KEYUTIL;
var merge = require("node.extend");
var common = require("asn1js/org/pkijs/common");
var _asn1js = require("asn1js");
var _pkijs = require("pkijs");
var _x509schema = require("pkijs/org/pkijs/x509_schema");
var hash = require(__dirname+"/hash");


// constants
const SHA2 = 'SHA2';
const SHA3 = 'SHA3';
export const TCertEncTCertIndex:string = "1.2.3.4.5.6.7";
const NonceSize:number = 24;
const AESKeyLength:number = 32;
const HMACKeyLength:number = 32;
const BlockSize:number = 16;

const GCMTagSize:number = 16;
const GCMStandardNonceSize:number = 12;

const ECIESKDFOutput = 512; // bits
const IVLength = 16; // bytes
const AESBlockLength = 16;

const CURVE_P_256_Size = 256;
const CURVE_P_384_Size = 384;

// variables
// #region Merging function/object declarations for ASN1js and PKIjs
var asn1js = merge(true, _asn1js, common);
var x509schema = merge(true, _x509schema, asn1js);
var pkijs_1 = merge(true, _pkijs, asn1js);
var pkijs = merge(true, pkijs_1, x509schema);

export class Crypto {

    private hashAlgorithm:string;
    private level:number;
    private curveName:string;
    private suite:string;

    constructor(hashAlgorithm:string, level:number) {
        hashAlgorithm = hashAlgorithm.toUpperCase();
        if (hashAlgorithm == SHA2) {
            throw Error("SHA2 is not yet supported");
        } else if (hashAlgorithm == SHA3) {
            this.hashAlgorithm = SHA3;
        } else {
            throw Error("unknown hash algorithm: " + hashAlgorithm);
        }
        this.checkLevel(level);
        this.level = level;
        this.suite = hashAlgorithm.toLowerCase() + '-' + this.level;
        if (level == CURVE_P_256_Size) {
            this.curveName = "secp256r1"
        } else if (level == CURVE_P_384_Size) {
            this.curveName = "secp384r1";
        } else{
            throw new Error("Invalid security level: " + level);
        }
    }

    GenerateNonce() {
        return crypto.randomBytes(NonceSize);
    }

    UnmarshallChainKey(chainKey:any) {
        // enrollChainKey is a PEM. Extract the key from it.
        var pem = new Buffer(chainKey, 'hex').toString();
        debug("ChainKey %s", pem);
        var chainKey = KEYUTIL.getHexFromPEM(pem, 'ECDSA PUBLIC KEY');
        // debug(chainKey);
        var certBuffer = this.toArrayBuffer(new Buffer(chainKey, 'hex'));
        var asn1 = pkijs.org.pkijs.fromBER(certBuffer);
        // debug('asn1:\n', asn1);
        var cert;
        cert = new pkijs.org.pkijs.simpl.PUBLIC_KEY_INFO({schema: asn1.result});
        // debug('cert:\n', JSON.stringify(cert, null, 4));

        var ab = new Uint8Array(cert.subjectPublicKey.value_block.value_hex);
        var ecdsaChainKey = this.keyFromPublic(ab, 'hex');

        return ecdsaChainKey
    }

    toArrayBuffer(buffer:any) {
        var ab = new ArrayBuffer(buffer.length);
        var view = new Uint8Array(ab);
        for (var i = 0; i < buffer.length; ++i) {
            view[i] = buffer[i];
        }
        return ab;
    }

    toBuffer(buffer:any) {
        var b = new Buffer(buffer.length);
        for (var i = 0; i < buffer.length; ++i) {
            b[i] = buffer.charCodeAt(i)
        }
        return b;
    }

    sign(key, msg) {
        var curve;
        var hash;

        // select curve and hash algo based on level
        switch (this.level) {
            case 256:
                curve = elliptic.curves['p256'];
                hash = sha3_256;
                break;
            case 384:
                curve = elliptic.curves['p384'];
                hash = sha3_384;
                break;
        }

        var ecdsa = new EC(curve);
        var signKey = ecdsa.keyFromPrivate(key, 'hex');
        var sig = ecdsa.sign(new Buffer(hash(msg), 'hex'), signKey);
        debug('ecdsa signature: ', sig)
        return sig;

    };

    keyFromPrivate(key, encoding) {

        // select curve and hash algo based on level
        var curve;
        switch (this.level) {
            case 256:
                curve = elliptic.curves['p256'];
                break;
            case 384:
                curve = elliptic.curves['p384'];
                break;
        }
        ;

        var keypair = new EC(curve).keyFromPrivate(key, encoding);
        ;
        debug('keypair: ', keypair)
        return keypair;
    };

    keyFromPublic(key, encoding) {

        var curve;
        //select curve and hash algo based on level
        switch (this.level) {
            case 256:
                curve = elliptic.curves['p256'];
                break;
            case 384:
                curve = elliptic.curves['p384'];
                break;
        }
        ;

        var publicKey = new EC(curve).keyFromPublic(key, encoding);
        // debug('publicKey: [%j]', publicKey);
        return publicKey;
    };

    generateKeyPair() {

        var curve;
        // select curve and hash algo based on level
        switch (this.level) {
            case 256:
                curve = elliptic.curves['p256'];
                break;
            case 384:
                curve = elliptic.curves['p384'];
                break;
        }
        ;

        var keypair = new EC(curve).genKeyPair();
        debug('keypair: ', keypair)
        return keypair;
    };

    GenerateECDSAKeyPair() {
        var curve;
        // select curve and hash algo based on level
        switch (this.level) {
            case 256:
                curve = "secp256r1";
                break;
            case 384:
                curve = "secp384r1";
                break;
        };
        return KEYUTIL.generateKeypair("EC", curve);
    };

    ECIESKeyGen() {
        var curve;
        // select curve and hash algo based on level
        switch (this.level) {
            case 256:
                curve = "secp256r1";
                break;
            case 384:
                curve = "secp384r1";
                break;
        };
        return KEYUTIL.generateKeypair("EC", curve);
    }

    ECIESEncryptECDSA(ecdsaRecipientPublicKey, msg): Buffer {
        var self = this;
        var EC = elliptic.ec;
        //var curve = elliptic.curves['p'+level];
        var ecdsa = new EC('p' + self.level);

        // Generate ephemeral key-pair
        var ephKeyPair = KEYUTIL.generateKeypair("EC", this.curveName);
        var ephPrivKey = ecdsa.keyFromPrivate(ephKeyPair.prvKeyObj.prvKeyHex, 'hex');
        var Rb = ephKeyPair.pubKeyObj.pubKeyHex;

        // Derive a shared secret field element z from the ephemeral secret key k
        // and convert z to an octet string Z
        // debug("ecdsa.keyFromPublic=%s", util.inspect(ecdsaRecipientPublicKey));//XXX
        var Z = ephPrivKey.derive(ecdsaRecipientPublicKey.pub);
        // debug('[Z]: %j', Z);
        var kdfOutput = self.hkdf(Z.toArray(), ECIESKDFOutput, null, null);
        // debug('[kdfOutput]: %j', new Buffer(new Buffer(kdfOutput).toString('hex'), 'hex').toString('hex'));

        var aesKey = kdfOutput.slice(0, AESKeyLength);
        var hmacKey = kdfOutput.slice(AESKeyLength, AESKeyLength + HMACKeyLength);
        // debug('[Ek] ', new Buffer(aesKey, 'hex'));
        // debug('[Mk] ', new Buffer(hmacKey, 'hex'));

        var iv = crypto.randomBytes(IVLength);
        var cipher = crypto.createCipheriv('aes-256-cfb', new Buffer(aesKey), iv);
        // debug("MSG %j: ", msg);
        var encryptedBytes = cipher.update(msg);
        // debug("encryptedBytes: ",JSON.stringify(encryptedBytes));
        var EM = Buffer.concat([iv, encryptedBytes]);
        var D = self.hmac(hmacKey, EM);

        // debug('[Rb] ', new Buffer(Rb,'hex').toString('hex')+" len="+Rb.length);
        // debug('[EM] ', EM.toString('hex'));
        // debug('[D] ', new Buffer(D).toString('hex'));

        return Buffer.concat([new Buffer(Rb, 'hex'), EM, new Buffer(D)]);
    }

    ECIESEncrypt(recipientPublicKey, msg) {
        var level = recipientPublicKey.ecparams.keylen;
        // debug("=============> %d", level);
        var EC = elliptic.ec;
        var curve = elliptic.curves["p" + level];
        // debug("=============> curve=%s", util.inspect(curve));
        var ecdsa = new EC(curve);

        return this.ECIESEncryptECDSA(ecdsa.keyFromPublic(recipientPublicKey.pubKeyHex, 'hex'), msg)
    }

    ECIESDecrypt(recipientPrivateKey, cipherText) {
        var self = this;
        // debug("recipientPrivateKey=%s", util.inspect(recipientPrivateKey));//XXX
        var level = recipientPrivateKey.ecparams.keylen;
        var curveName = recipientPrivateKey.curveName;
        // debug("=============> %d", level);
        self.checkLevel(level);
        //cipherText = ephemeralPubKeyBytes + encryptedTokBytes + macBytes
        //ephemeralPubKeyBytes = first ((384+7)/8)*2 + 1 bytes = first 97 bytes
        //hmac is sha3_384 = 48 bytes or sha3_256 = 32 bytes
        var Rb_len = Math.floor((level + 7) / 8) * 2 + 1;
        var D_len = level >> 3;
        var ct_len = cipherText.length;

        if (ct_len <= Rb_len + D_len)
            throw new Error("Illegal cipherText length: " + ct_len + " must be > " + (Rb_len + D_len));

        var Rb = cipherText.slice(0, Rb_len);  // ephemeral public key bytes
        var EM = cipherText.slice(Rb_len, ct_len - D_len);  // encrypted content bytes
        var D = cipherText.slice(ct_len - D_len);

        // debug("Rb :\n", new Buffer(Rb).toString('hex'));
        // debug("EM :\n", new Buffer(EM).toString('hex'));
        // debug("D  :\n", new Buffer(D).toString('hex'));

        var EC = elliptic.ec;
        //var curve = elliptic.curves['p'+level];
        var ecdsa = new EC('p' + level);

        //convert bytes to usable key object
        var ephPubKey = ecdsa.keyFromPublic(new Buffer(Rb, 'hex'), 'hex');
        //var encPrivKey = ecdsa.keyFromPrivate(ecKeypair2.prvKeyObj.prvKeyHex, 'hex');
        var privKey = ecdsa.keyFromPrivate(recipientPrivateKey.prvKeyHex, 'hex');
        // debug('computing Z...', privKey, ephPubKey);

        var Z = privKey.derive(ephPubKey.pub);
        // debug('Z computed', Z);
        // debug('secret:  ', new Buffer(Z.toArray(), 'hex'));
        var kdfOutput = self.hkdf(Z.toArray(), ECIESKDFOutput, null, null);
        var aesKey = kdfOutput.slice(0, AESKeyLength);
        var hmacKey = kdfOutput.slice(AESKeyLength, AESKeyLength + HMACKeyLength);
        // debug('secret:  ', new Buffer(Z.toArray(), 'hex'));
        // debug('aesKey:  ', new Buffer(aesKey, 'hex'));
        // debug('hmacKey: ', new Buffer(hmacKey, 'hex'));

        var recoveredD = self.hmac(hmacKey, EM);
        debug('recoveredD:  ', new Buffer(recoveredD).toString('hex'));

        if (D.compare(new Buffer(recoveredD)) != 0) {
            // debug("D="+D.toString('hex')+" vs "+new Buffer(recoveredD).toString('hex'));
            throw new Error("HMAC verify failed");
        }
        var iv = EM.slice(0, IVLength);
        var cipher = crypto.createDecipheriv('aes-256-cfb', new Buffer(aesKey), iv);
        var decryptedBytes = cipher.update(EM.slice(IVLength));
        // debug("decryptedBytes: ",new Buffer(decryptedBytes).toString('hex'));
        return decryptedBytes;
    }

    aesCFBDecryt(key, encryptedBytes) {

        var iv = crypto.randomBytes(IVLength);
        var aes = new aesjs.ModeOfOperation.cfb(key, iv, IVLength);

        debug("encryptedBytes: ", encryptedBytes);

        //need to pad encryptedBytes to multiples of 16
        var numMissingBytes = IVLength - (encryptedBytes.length % AESBlockLength);
        debug("missingBytes: ", numMissingBytes);

        if (numMissingBytes > 0) {
            encryptedBytes = Buffer.concat([encryptedBytes, new Buffer(numMissingBytes)]);
        }

        debug("encryptedBytes: ", encryptedBytes);

        var decryptedBytes = aes.decrypt(encryptedBytes);

        return decryptedBytes.slice(IVLength, decryptedBytes.length - numMissingBytes);

    }

    hkdf(ikm, keyBitLength, salt, info) {

        var hash;
        var hashSize;

        switch (this.suite) {
            case "sha3-256":
                debug("Using sha3-256");
                hash = hash.hash_sha3_256;
                hashSize = 32;
                break;
            case "sha3-384":
                debug("Using sha3-384");
                hash = hash.hash_sha3_384;
                hashSize = 48;
                break;
            case "sha2-256":
                debug("Using sha3-256");
                //hash = hash_sha2_256;
                hashSize = 32;
                break;
        }
        ;

        if (!salt)
            salt = zeroBuffer(hashSize);

        if (!info)
            info = "";

        var key = this.hkdf2(bytesToBits(new Buffer(ikm)), keyBitLength, bytesToBits(salt), info, hash);

        return bitsToBytes(key);

    }

    /** HKDF with the specified hash function.
     * @param {bitArray} ikm The input keying material.
     * @param {Number} keyBitLength The output key length, in bits.
     * @param {String|bitArray} salt The salt for HKDF.
     * @param {String|bitArray} info The info for HKDF.
     * @param {Object} [Hash=sjcl.hash.sha256] The hash function to use.
     * @return {bitArray} derived key.
     */
    private hkdf2(ikm, keyBitLength, salt, info, Hash) {
        var hmac, key, i, hashLen, loops, curOut, ret = [];

        // Hash = Hash || sjcl.hash.sha256;
        if (typeof info === "string") {
            info = sjcl.codec.utf8String.toBits(info);
        } else if (!info) {
            info = sjcl.codec.utf8String.toBits('');
        }
        if (typeof salt === "string") {
            salt = sjcl.codec.utf8String.toBits(salt);
        } else if (!salt) {
            salt = [];
        }

        hmac = new sjcl.misc.hmac(salt, Hash);
        //key = hmac.mac(ikm);
        hmac.update(ikm);
        key = hmac.digest();
        // debug("prk: %j", new Buffer(bitsToBytes(key)).toString('hex'));
        hashLen = sjcl.bitArray.bitLength(key);

        loops = Math.ceil(keyBitLength / hashLen);
        if (loops > 255) {
            throw new sjcl.exception.invalid("key bit length is too large for hkdf");
        }

        curOut = [];
        for (i = 1; i <= loops; i++) {
            hmac = new sjcl.misc.hmac(key, Hash);
            hmac.update(curOut);
            hmac.update(info);
            // debug('sjcl.bitArray.partial(8, i): %j', sjcl.bitArray.partial(8, i));
            hmac.update(bytesToBits([i]));

            // hmac.update([sjcl.bitArray.partial(8, i)]);
            curOut = hmac.digest();
            ret = sjcl.bitArray.concat(ret, curOut);
        }
        return sjcl.bitArray.clamp(ret, keyBitLength);
    }

    private CBCDecrypt(key, bytes) {
        debug('key length: ', key.length);
        debug('bytes length: ', bytes.length);
        var iv = bytes.slice(0, BlockSize);
        debug('iv length: ', iv.length);
        var encryptedBytes = bytes.slice(BlockSize);
        debug('encrypted bytes length: ', encryptedBytes.length);

        var decryptedBlocks = [];
        var decryptedBytes;

        // CBC only works with 16 bytes blocks
        if (encryptedBytes.length > BlockSize) {
            //CBC only support cipertext with length Blocksize
            var start = 0;
            var end = BlockSize;
            while (end <= encryptedBytes.length) {
                var aesCbc = new aesjs.ModeOfOperation.cbc(key, iv);
                debug('start|end', start, end);
                var encryptedBlock = encryptedBytes.slice(start, end);
                var decryptedBlock = aesCbc.decrypt(encryptedBlock);
                debug('decryptedBlock: ', decryptedBlock);
                decryptedBlocks.push(decryptedBlock);
                //iv for next round equals previous block
                iv = encryptedBlock;
                start += BlockSize;
                end += BlockSize;
            }
            ;

            decryptedBytes = Buffer.concat(decryptedBlocks);
        }
        else {
            var aesCbc = new aesjs.ModeOfOperation.cbc(key, iv);
            decryptedBytes = aesCbc.decrypt(encryptedBytes);
        }

        debug('decrypted bytes: ', JSON.stringify(decryptedBytes));

        return decryptedBytes;

    };

    public CBCPKCS7Decrypt(key, bytes) {

        var decryptedBytes, unpaddedBytes;

        decryptedBytes = this.CBCDecrypt(key, bytes);
        unpaddedBytes = this.PKCS7UnPadding(decryptedBytes);

        return unpaddedBytes;
    };

    private PKCS7UnPadding(bytes) {

        //last byte is the number of padded bytes
        var padding = bytes.readUInt8(bytes.length - 1);
        debug('padding: ', padding);
        //should check padded bytes, but just going to extract
        var unpadded = bytes.slice(0, bytes.length - padding);
        debug('unpadded bytes: ', JSON.stringify(unpadded));
        return unpadded;
    };

    AESKeyGen() {
        return crypto.randomBytes(AESKeyLength);
    }

    hmac(key, bytes) {
        var self = this;
        debug('key: ', JSON.stringify(key));
        debug('bytes: ', JSON.stringify(bytes));
        var hash, hashSize;
        switch (self.level) {
            case 256:
                hash = hash.hash_sha3_256;
                hashSize = 32;
                break;
            case 384:
                hash = hash.hash_sha3_384;
                hashSize = 48;
                break;
        }

        var hmac = new sjcl.misc.hmac(bytesToBits(key), hash);
        hmac.update(bytesToBits(bytes));
        var result = hmac.digest();
        debug("result: ", bitsToBytes(result));
        return bitsToBytes(result);
    }

    hash(bytes) {
        var self = this;
        debug('bytes: ', JSON.stringify(bytes));
        var hash, hashSize;
        switch (self.level) {
            case 256:
                hash = sha3_256;
                hashSize = 32;
                break;
            case 384:
                hash = sha3_384;
                hashSize = 48;
                break;
        }
        return hash(bytes);
    }

    decryptResult(key, ct) {
        // debug('Decrypt result..');

        // debug('ciphertext %j', ct);
        // debug('CT.length %j', ct.length);
        //
        // debug('KEY %j', key);
        // debug('IV %j', ct.slice(0, GCMStandardNonceSize));
        // debug('IV length %j', ct.slice(0, GCMStandardNonceSize).length);
        // debug('AUTH %j', ct.slice(ct.length - GCMTagSize));
        // debug('AUTH length %j', ct.slice(ct.length - GCMTagSize).length);
        // debug('CONTENT %s', ct.slice(GCMStandardNonceSize, ct.length - GCMTagSize));

        let decipher = crypto.createDecipheriv('aes-256-gcm', key, ct.slice(0, GCMStandardNonceSize));
        decipher.setAuthTag(ct.slice(ct.length - GCMTagSize));
        let dec = decipher.update(
            ct.slice(GCMStandardNonceSize, ct.length - GCMTagSize).toString('hex'),
            'hex', 'hex'
        );
        dec += decipher.final('hex');
        return dec;
    }

    hmacAESTruncated(key, bytes) {
        var res = this.hmac(key, bytes);
        return res.slice(0, AESKeyLength);
    }

    private checkLevel(level) {
        if (level != 256 && level != 384)
            throw new Error("Illegal level: " + level + " - must be either 256 or 384");
    }

    PrivateKeyHextoASN1(prvKeyHex:string) {
        var Ber = require('asn1').Ber;
        var sk = new Ber.Writer();
        sk.startSequence();
        sk.writeInt(1);
        sk.writeBuffer(new Buffer(prvKeyHex, 'hex'), 4);
        sk.writeByte(160);
        sk.writeByte(7);
        if (this.level == CURVE_P_384_Size ) {
            // OID of P384
            sk.writeOID('1.3.132.0.34');
        } else if (this.level == CURVE_P_256_Size) {
            // OID of P256
            sk.writeOID('1.2.840.10045.3.1.7');
        } else {
            throw Error("Not supported. Level " + this.level)
        }
        sk.endSequence();
        return sk.buffer;
    }

    /**
     * Get the security level
     * @returns The security level
     */
    getSecurityLevel():number {
        return this.level;
    }

    /**
     * Set the security level
     * @params securityLevel The security level
     */
    setSecurityLevel(securityLevel:number):void {
        this.level = securityLevel;
    }

    /**
     * Get the hash algorithm
     * @returns {string} The hash algorithm
     */
    getHashAlgorithm():string {
        return this.hashAlgorithm;
    }

    /**
     * Set the hash algorithm
     * @params hashAlgorithm The hash algorithm ('SHA2' or 'SHA3')
     */
    setHashAlgorithm(hashAlgorithm:string):void {
        this.hashAlgorithm = hashAlgorithm;
    }


}  // end Crypto class

export class X509Certificate {

    private _cert:any;

    constructor(buffer) {
        debug('cert:', JSON.stringify(buffer));
        // convert certBuffer to arraybuffer
        var certBuffer = _toArrayBuffer(buffer);
        // parse the DER-encoded buffer
        var asn1 = pkijs.org.pkijs.fromBER(certBuffer);
        this._cert = {};
        try {
            this._cert = new pkijs.org.pkijs.simpl.CERT({schema: asn1.result});
            debug('decoded certificate:\n', JSON.stringify(this._cert, null, 4));
        } catch (ex) {
            debug('error parsing certificate bytes: ', ex)
            throw ex;
        }
    }

    criticalExtension(oid) {
        var ext;
        debug('oid: ', oid);
        this._cert.extensions.some(function (extension) {
            debug('extnID: ', extension.extnID);
            if (extension.extnID === oid) {
                ext = extension;
                return true;
            }
        });
        debug('found extension: ', ext);
        debug('extValue: ', _toBuffer(ext.extnValue.value_block.value_hex));
        return _toBuffer(ext.extnValue.value_block.value_hex);
    }

} // end X509Certificate class

function bitsToBytes(arr) {
    var out = [], bl = sjcl.bitArray.bitLength(arr), i, tmp;
    for (i = 0; i < bl / 8; i++) {
        if ((i & 3) === 0) {
            tmp = arr[i / 4];
        }
        out.push(tmp >>> 24);
        tmp <<= 8;
    }
    return out;
}

/** Convert from an array of bytes to a bitArray. */
function bytesToBits(bytes) {
    var out = [], i, tmp = 0;
    for (i = 0; i < bytes.length; i++) {
        tmp = tmp << 8 | bytes[i];
        if ((i & 3) === 3) {
            out.push(tmp);
            tmp = 0;
        }
    }
    if (i & 3) {
        out.push(sjcl.bitArray.partial(8 * (i & 3), tmp));
    }
    return out;
}

function hexToBytes(hex) {
    for (var bytes = [], c = 0; c < hex.length; c += 2)
        bytes.push(parseInt(hex.substr(c, 2), 16));
    return bytes;
};

function zeroBuffer(length) {
    var buf = new Buffer(length);
    buf.fill(0);
    return buf
};

// utility function to convert Node buffers to Javascript arraybuffer
function _toArrayBuffer(buffer) {
    var ab = new ArrayBuffer(buffer.length);
    var view = new Uint8Array(ab);
    for (var i = 0; i < buffer.length; ++i) {
        view[i] = buffer[i];
    }
    return ab;
};

// utility function to convert Javascript arraybuffer to Node buffers
function _toBuffer(ab) {
    var buffer = new Buffer(ab.byteLength);
    var view = new Uint8Array(ab);
    for (var i = 0; i < buffer.length; ++i) {
        buffer[i] = view[i];
    }
    return buffer;
};
