[Issue #961](https://github.com/hyperledger/fabric/issues/961)

This section will document the design of the access control mechanisms for chaincode developers.

The process will be driven by demonstrating a test driven approach using the following functional matrix as the outline of phases of RBAC/ABAC maturity.

RBAC/ABAC Functionality | CC to CC invoke | Attr Based | Role based | TCert based | User Defined Membership Services | Doable with current master| Note | Link to behave feature |
--- | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: |
access control based on TCerts (No attributes, not role based) |  |   |   | X | X | X | asset_mgt.go  |  |
Role based access control using TCerts w/o using attributes    |  |   | X | X | X | X |  TBD, extension of asset_mgt.go |  |
Attribute based access control using Tcerts with Attributes    |  | X | X |   |   |   |  extension of asset_mgt_with_roles.go |  |
attribute access control with User Defined membership services |  |   |   |   | X |   |   |   |
All of above | X | X | X | X | X |  |   |    |
