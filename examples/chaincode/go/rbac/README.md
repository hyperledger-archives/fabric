# Hyperledger Fabric - Role-based Access Control

## Overview

The *rbac* chaincode (*rbac.go*) is a very simple chaincode designed to show how to exercise *robe-based access control* at the chaincode level by leveraging the techniques described in [Application ACLs](https://github.com/hyperledger/fabric/blob/master/docs/tech/application-ACL.md)

The chaincode exposes the following functions:

1. *init(user)*: Initialize the chaincode assigning to *user* the role of *administrator*;
2. *addRole(user, role)*: Assigns the role *role* to *user*. 
Notice that, this function can be invoked only by an administrator;
3. *write(data)*: Set the current chaincode's state to *data*.
Notice that this function can be invoked only by users who have the role *writer*;
4. *read()*: Returns the current chaincode's state.
Notice that this function can be invoked only by users who have the role *reader*;

In the following subsections, we will describe in more detail each function.

## *init(user)*

This function initialize the chaincode by assigning to *user* the role of administrator. The function is invoked automatically at deploy time.

When generating the deploy transaction, the chaincode deployer must specify the administrator of the chaincode by setting the transaction metadata to 
the DER (Distinguished Encoding Rules) certificate encoding of one of the administrator ECert/TCert. 

For simplicity, there is only one administrator.

A possible work-flow could be the following:

1. Alice is the deployer of the chaincode;
2. Alice wants to assign the administrator role to Bob;
3. Alice obtains, via an out-of-band channel, a TCert of Bob, let us call this certificate *BobCert*;
4. Alice constructs a deploy transaction, as described in [Application ACLs](https://github.com/hyperledger/fabric/blob/master/docs/tech/application-ACL.md),  setting the transaction metadata to *DER(BobCert)*.
5. Alice submits the transaction to the fabric network.

Notice that Alice can assign to herself the role of administrator.

## *addRole(user, role)*

This function assigns the role *role* to *user*.  For simplicity, *role* can be any string (the identifier of the role, for example) and *user* is a TCert/ECert of the party the role *role* is assigned to.

Notice that, this function can only be invoked by the administrator of the chaincode that has been defined at deploy time during the chaincode initialization.

A possible work-flow could be the following:

1. Bob is the administrator of the chaincode;
2. Bob wants to assign the role 'writer' to Charlie;
3. Bob obtains, via an out-of-band channel, a TCert of Charlie, let us call this certificate *CharlieCert*;
4. Bob constructs an execute transaction:
	1. To invoke the *addRole* function passing as parameters *('writer', DER(CharlieCert))*, and
	2. by setting the metadata field to a signature of a message that consists of the concatenation of *BobCert||chaincodeInput||binding* generated using the signing key corresponding to *BobCert*. Please, read [Application ACLs](https://github.com/hyperledger/fabric/blob/master/docs/tech/application-ACL.md) for more details on *chaincodeInput* and *binding*.
5. Bob submits the transaction to the fabric network.
2. Bob wants to assign the role 'reader' to Dave;
3. Bob obtains, via an out-of-band channel, a TCert of Dave, let us call this certificate *DaveCert*;
4. Bob constructs an execute transaction as above to invoke the *addRole* function passing as parameters *('reader', DER(DaveCert))*.
5. Bob submits the transaction to the fabric network.

## *write(data)*

This function sets the current chaincode's state to *data*.
Notice that this function ca be invoked only by users who have the role *writer*;

A possible work-flow could be the following:

1. Charlie wants to set chaincode's state to *10*;
2. Charlie constructs an execute transaction:
	1. To invoke the *write* function passing as parameters *(data)*, and
	2. by setting the metadata field to a signature of a message that consists of the concatenation of *CharlieCert||chaincodeInput||binding* generated using the signing key corresponding to *CharlieCert*.
3. Charlie submits the transaction to the fabric network.

## *read()*

This function returns the current chaincode's state.
Notice that this function can be invoked only by users who have the role *reader*;

A possible work-flow could be the following:

1. Dave wants to read the current chaincode's state;
2. Dave constructs a query transaction:
	1. To query the *read* function, and
	2. by setting the metadata field to a signature of a message that consists of the concatenation of *DaveCert||chaincodeInput||binding* generated using the signing key corresponding to *DaveCert*.
3. Dave submits the transaction to the fabric network and waits for the result.
