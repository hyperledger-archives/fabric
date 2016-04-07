## Usage

&nbsp;
#####What are the expected performance figures for the fabric?
The performance of a chain network depends on multiple factors, such as the encryption method used, transaction message size, security level set, business logic running, number of validators, proximity of the validating nodes, the consensus algorithm deployed, etc.

The current performance goal for the fabric is to achieve 100,000 transactions per second in a standard production environment of approximately 15 validating nodes running in close proximity. The team is committed to continuously improving the performance and the scalability of the system.

&nbsp;
##### Do I have to own a validating node to transact on a chain network?
No. You can still transact on a chain network by owning a non-validating node (NV-node).

Although transactions initiated by non-validating nodes will eventually get forwarded to their validating peers (so they can be validated in consensus processing), NV-nodes establish their own connections to the membership service module and can therefore package transactions independently. This capability allows NV-node owners to independently register and manage certificates on their own behalf for their own clients, which is a powerful feature that permits NV-node owners to create custom-built applications for their own clients while managing their client certificates on their own.

In addition, each non-validating node retains a full copy of the ledger, so that NV-node owners can always query ledger data locally instead of directing such calls to remote validating nodes owned by someone else.
