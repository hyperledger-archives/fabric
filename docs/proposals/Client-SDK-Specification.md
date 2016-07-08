[Issue #960](https://github.com/hyperledger/fabric/issues/960)

The current implementation of these functions requires a non-validating peer (NVP), which provides transaction, event, and some implicit wallet management APIs. 
![Current SDK](https://github.com/hyperledger/fabric/blob/master/docs/wiki-images/sdk-current.png)

As shown above, the app calls NVP through REST API (eg invoke transaction). The request arrives at the `HTTP` service, which gRPCs to the `devops` service. The `devops` service constructs the transaction, signs it, and gRPCs it to VP (or NVP where it runs on).

To address the above stories, which covers client API, thin client, transaction confidentiality, and wallet management, we may group these functions in an SDK providing native APIs for application to use on 1 side and gRPC to Endorsers (new name for validator without consensus) directly on the other side without having to go through NVP. The key motivation for this design is to reduce the latency going through REST and multiple gRPC hops. Since this model allows the application to use the SDK directly in the same process, it provides better security in managing user-certificate mapping with user-authentication.

The SDK may be predicted as follow:
![Future SDK](https://github.com/hyperledger/fabric/blob/master/docs/wiki-images/sdk-future.png)

Basically this SDK rips out the capabilities from NVP and rolls them into library. Of course with additional work on the wallet management as specified below, but the capabilities on transaction and event handling should stay in tact. The native API layer provide Node.js and Java bindings through gRPC to the SDK library.

The alternative model would be to implement the SDK natively in Node.js and Java and gRPC to the Endorsers. This would avoid an extra hop in gRPC, but the cost of reimplementing the NVP capabilities existing in Go is quite high, especially doing that in 2 languages. Another alternative would be to look into Go binding to Java and JavaScript, which would have to go through C and quite convoluted code.

May 11/2016 update:
------------------
The alternative model described above is preferred and now being worked on.  Even though it is more work, it is better for the end user to have a pure language model.  The 1st SDK client being worked on is for node.js and can be found in the fabric/sdk/node directory.  There is a README.md with an overall description of the API.  The interactions with membership services is working, and the transaction path is currently being worked on.  This should be complete in a couple of weeks, so you can watch fabric/sdk/node for changes.

Once this is complete, hopefully others can help with the client SDK for other languages, following the node.js SDK as a model.  The desire is that other languages look much like the node.js API, except where it makes sense to diverge for some language-specific reason.
 