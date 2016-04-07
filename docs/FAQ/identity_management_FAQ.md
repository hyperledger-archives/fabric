## Identity Management (Membership Service)
&nbsp;
##### What is unique about the fabric's Membership Service module?
The design and implementation of the Membership Service module encompass many of the latest advances in cryptography, which we believe make it stand apart from other alternatives.

In addition to supporting generally-expected requirements such as preserving the privacy of transactions and making them auditable, the fabric's membership service module also introduces the concept of enrollment and transaction certificates. Infinite number of transaction certificates can be generated from their parent enrollment certificates (issued to validated users). This design ensures that asset tokens (which can be represented by transaction certificates) can only be created by verified owners, and private keys of asset tokens can be regenerated if lost.

Furthermore, the design of the system allows transaction certificates to both expire and be revoked, which allows issuers to have greater control over the asset tokens that they issued on a distributed chain.

Finally, like most other modules on the fabric, you can always replace the default membership service implementation with another option.


&nbsp;
##### Does its Membership Service make the fabric a centralized solution?

No, because the fabric's Membership Service does not provide, deploy, validate, or execute transactions or business logic. The only role of the Membership Service is to issue digital certificates to validated entities that want to participate in the network. The service is not aware of how or when these certificates are used in any particular network.

The fabric's Membership Service does, however, serve as the central regulator of the networks that it services, because the certificates that the service issues are used by the networks to regulate and manage their users.
