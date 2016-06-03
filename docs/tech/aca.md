# Attribute Certificate Authority

To support Attribute Based Access Control (ABAC) the user has to pass a set of attributes during TCert creation, these attributes will be used during transaction execution or query to determine whether the user can or cannot execute a specific chaincode. A mechanism to validate the ownership of attributes is required in order to prove if the attributes passed by the user are correct. The Attribute Certificate Authority (ACA) has the responsibility of validating attributes and in response it returns an Attribute Certificate (ACert) with the valid attribute values.

ACA will have a database to hold attributes for each user and affiliation. This DB will be encrypted.

1. userID _(The id passed by the user during enrollment)_
2. affiliation _(The entity which the user is affiliated to)_
3. attribute Key _(The key used to look for the attribute, e.g. 'position')_
4. attribute value _(The value of the attribute, e.g. 'software engineer')_
5. valid from _(The start of the attribute's validity period)_
6. valid to _(The end of the attribute's validity period)_

## FLOW

![ACA flow](https://raw.githubusercontent.com/andresgaragiola/fabric/ABAC/docs/images/aca_flow.jpg)

### During enrollment

1. The user requests an Enrollment Certificate (ECert) to ECA
2. ECA creates the ECert and responds to the user with it.
3. ECA issues a fetch request under TLS to the ACA passing the newly generated ECert as a parameter. This request is signed with the ECA's private key.
4. The request triggers ACA asynchronous mechanism that fetches attributes' values from external sources and populates the attributes database (in the current implementation attributes are loaded from an internal configuration file).

### During TCert generation

1. When the user needs TCerts to create a new transaction it requests a batch of TCerts to the TCA, and provides the following:
   * The batch size (i.e. how many TCerts the user is expecting)
   * Its ECert
   * A list of attributes (e.g. Company:IBM, Position: Software Engineer)
2. Under TLS TCA sends a RequestAttributes() to ACA to verify if the user is in possession of those attributes. This request is signed with TCA's private key and it contains:
   * User's ECert
   * A key/pair list of attributes where the value of the attribute is hashed:
     * Company: Hash(IBM)
     * Position: Hash(Software Engineer)
3. ACA performs a query to the internal attributes database and there are three possible scenarios***:
     a. The user does not have any of the specified attributes – An error is returned.
     b. The user has all the specified attributes – An X.509 certificate (ACert) with all the specified attributes and the ECert public key is returned.
     c. The user has a subset of the requested attributes – An X.509 certificate (ACert) with just the subset of the specified attributes and the ECert public key is returned.
3.  TCA checks the validity period of the ACert's attributes and updates the list by eliminating those that are expired. Then for scenarios b and c from the previous item it checks how many (and which ones) of the attributes the user will actually receive inside each TCert. This information needs to be returned to the user in order to decide whether the TCerts are useful or if further actions needs to be performed (i.e. issue a refreshAttributes command and request a new batch, throw an error or make use of the TCerts as they are).
4.  TCA could have other criteria to update the valid list of attributes.
5.  TCA creates the batch of TCerts. Each TCert contains the valid attributes encrypted with keys derived from the Prekey tree (each key is unique per attribute, per TCert and per user).
6.  TCA returns the batch of TCerts to the user along with a root key (Prek0) from which each attribute encryption key was derived. There is a Prek0 per TCert. All the TCerts in the batch have the same attributes and the validity period of the TCerts is the same for the entire batch.

*** _In the current implementation an attributes refresh is executed automatically before this step, but once the refresh service is implemented the user will have the responsibility of keeping his/her attributes updated by invoking this method._

### gRPC ACA API

1. FetchAttributes

```
    rpc FetchAttributes(ACAFetchAttrReq) returns (ACAFetchAttrResp);

    message ACAFetchAttrReq {
        google.protobuf.Timestamp ts = 1;
        Cert eCert = 2;                  // ECert of involved user.
        Signature signature = 3;         // Signed using the ECA private key.
    }

    message ACAFetchAttrResp {
            enum StatusCode {
    	       SUCCESS = 000;
    	       FAILURE = 100;
	    }
        StatusCode status = 1;
    }
```

2. RequestAttributes

```
    rpc RequestAttributes(ACAAttrReq) returns (ACAAttrResp);

    message ACAAttrReq {
        google.protobuf.Timestamp ts = 1;
        Identity id = 2;
        Cert eCert = 3;                                // ECert of involved user.
        repeated TCertAttributeHash attributes = 4;    // Pairs attribute-key, attribute-value-hash
        Signature signature = 5;                       // Signed using the TCA private key.
    }

    message ACAAttrResp {
        enum StatusCode {
        	FULL_SUCCESSFUL     = 000;
           PARTIAL_SUCCESSFUL  = 001;
    	    NO_ATTRIBUTES_FOUND = 010;
    	    FAILURE	          = 100;
    	 }
        StatusCode status = 1;
        Cert cert = 2;                  // ACert with the owned attributes.
        Signature signature = 3;        // Signed using the ACA private key.
    }
```

3. RefreshAttributes

```
    rpc RefreshAttributes(ACARefreshReq) returns (ACARefreshResp);

    message ACARefreshAttrReq {
        google.protobuf.Timestamp ts = 1;
        Cert eCert = 2;                              // ECert of the involved user.
        Signature signature = 3;                     // Signed using enrollPrivKey
    }

    message ACARefreshAttrResp {
            enum StatusCode {
    	       SUCCESS = 000;
    	       FAILURE = 100;
	    }
        StatusCode status = 1;
    }
```

### Assumptions

1. An Attribute Certificate Authority (ACA) has been incorporated to the Membership Services internally to provide a trusted source for attribute values.
2. In the current implementation attributes are loaded from a configuration file (membersrvc.yml).
3. Refresh attributes service is not implemented yet, instead, attributes are refreshed in each RequestAttribute invocation.
