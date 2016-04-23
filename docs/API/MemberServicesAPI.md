# Certificate Authority API

Each of the CA services is split into two [GRPC](http://www.grpc.io) interfaces, namely a public one (indicated by a _P_ suffix) and an administrator one (indicated by an _A_ suffix).

## Enrollment Certificate Authority

The administrator interface of the ECA provides the following functions:

	service ECAA { // admin
	    rpc RegisterUser(RegisterUserReq) returns (Token);
	    rpc ReadUserSet(ReadUserSetReq) returns (UserSet);
	    rpc RevokeCertificate(ECertRevokeReq) returns (CAStatus); // not yet implemented
	    rpc PublishCRL(ECertCRLReq) returns (CAStatus); // not yet implemented
	}

The `RegisterUser` function allows you to register a new user by specifiying her name and roles in the `RegisterUserReq` structure.  If the user has not be registered before, the ECA registers the new user and returns a unique one-time password, which can be used by the user to request her enrollment certificate pair via the public interface of the ECA.  Otherwise an error is returned.

The `ReadUserSet` function allows only auditors to retrieve the list of users registered with the blockchain.

The public interface of the ECA provides the following functions:

	service ECAP { // public
	    rpc ReadCACertificate(Empty) returns (Cert);
	    rpc CreateCertificatePair(ECertCreateReq) returns (ECertCreateResp);
	    rpc ReadCertificatePair(ECertReadReq) returns (CertPair);
	    rpc ReadCertificateByHash(Hash) returns (Cert);
	    rpc RevokeCertificatePair(ECertRevokeReq) returns (CAStatus); // not yet implemented
	}

The `ReadCACertificate` function returns the certificate of the ECA itself.

The `CreateCertificatePair` functions allows a user to create and read her enrollment certificate pair.  For this, the user has to do two successive invocations of this functions.  Firstly, both the signature and encryption public keys have to be handed to the ECA together with the one-time password returned by the `RegisterUser` function invocation before.  The request has to be signed by the user's private signature key to demonstrate that the user is in possession of the private signature key indeed.  The ECA in return gives the user a challenge encrypted with the user's public encryption key.  The has to decrypt the challenge, thereby demonstrating that she is in possession of the private encryption key indeed, and re-issue the certificate creation request passing the decrypted challenge instead of the one-time password passed in the invocation.  If the challenge has been decrypted correctly, the ECA issues and returns the enrollment certificate pair for the user.

The `ReadCertificatePair` function allows any user of the blockchain to read the certificate pair of any other user of the blockchain.

The `ReadCertificatePairByHash` function allows any user of the blockchain to read a certificate from the ECA matching a given hash.

## Transaction Certificate Authority

The administrator interface of the TCA provides the following functions:

	service TCAA { // admin
	    rpc ReadCertificateSets(TCertReadSetsReq) returns (CertSets);
	    rpc RevokeCertificate(TCertRevokeReq) returns (CAStatus); // not yet implemented
	    rpc RevokeCertificateSet(TCertRevokeSetReq) returns (CAStatus); // not yet implemented
	    rpc PublishCRL(TCertCRLReq) returns (CAStatus); // not yet implemented
	}

The `ReadCertificateSets` function allows auditors only to read transaction certificate sets for some or all users of the blockchain.

The public interface of the TCA provides the following functions:

	service TCAP { // public
	    rpc ReadCACertificate(Empty) returns (Cert);
	    rpc CreateCertificate(TCertCreateReq) returns (TCertCreateResp);
	    rpc CreateCertificateSet(TCertCreateSetReq) returns (TCertCreateSetResp);
	    rpc ReadCertificate(TCertReadReq) returns (Cert);
	    rpc ReadCertificateSet(TCertReadSetReq) returns (CertSet);
	    rpc RevokeCertificate(TCertRevokeReq) returns (CAStatus); // not yet implemented
	    rpc RevokeCertificateSet(TCertRevokeSetReq) returns (CAStatus); // not yet implemented
	}

The `ReadCACertificate` function returns the certificate of the TCA itself.

The `CreateCertificate` function allows a user to create and retrieve a new transaction certificate.

The `CreateCertificateSet` function allows a user to create and retrieve a set of transaction certificates in a single call.

The `ReadCertificate` allows a user to retrieve a previously created transaction certificate.  This function can also be called by validators and auditors for any user of the blockchain.

The `ReadCertificateSet` allows a user to retrieve a previously created transaction certificate set.  This function can also be called by auditors for any user of the blockchain.

## TLS Certificate Authority

The administrator interface of the TLSCA provides the following functions:

	service TLSCAA { // admin
	    rpc RevokeCertificate(TLSCertRevokeReq) returns (CAStatus); not yet implemented
	}

The public interface of the TLSCA provides the following functions:

	service TLSCAP { // public
	    rpc ReadCACertificate(Empty) returns (Cert);
	    rpc CreateCertificate(TLSCertCreateReq) returns (TLSCertCreateResp);
	    rpc ReadCertificate(TLSCertReadReq) returns (Cert);
	    rpc RevokeCertificate(TLSCertRevokeReq) returns (CAStatus); // not yet implemented
	}

The `ReadCACertificate` function returns the certificate of the TLSCA itself.

The `CreateCertificate` function allows a user to create and retrieve a new TLS certificate.

The `ReadCertificate` function allows a user to retrieve a previously created TLS certificate.
