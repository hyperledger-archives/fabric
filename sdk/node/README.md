The Hyperledger Client ("hlc") SDK provides APIs through which a client may interact with a hyperledger blockchain.

## Terminology

* member  
  An identity for participating in the blockchain.  There are different types of members (users, peers, etc).

* member services  
  Services related to obtaining and managing members.

* registration  
   The act of adding a new member identity (with specific privileges) to the system.  
   This is done by an administrator, or more accurately, a member (called a registrar) with the 'registrar' privilege.  
   The registrar specifies the new member privileges when registering the new member.
   The output of registration is an enrollment secret (i.e. a one-time password).

* enrollment  
  The act of completing registration and obtaining credentials which are used to transact on the blockchain.  Enrollment may be done by the end user, in which case only the end user has access to his/her credentials.  Alternatively, the registrar may have delegated authority to act on behalf of the end user, in which case the registrar also performs enrollment for the user.
  
## Pluggability

These APIs have been designed to support two pluggable components.

1. Pluggable key value store which is used to retrieve and store keys associated with a member.

2. Pluggable member service which is used to register and enroll members.