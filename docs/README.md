## Notice regarding the Linux Foundation's Hyperledger project

The IBM Open Blockchain (OBC) project is our contribution to the Linux Foundation [Hyperledger](https://www.hyperledger.org/) project. We have made it available as open source to enable others to explore our architecture and design. IBM intends to engage rigorously in the Linux Foundation [Hyperledger](https://www.hyperledger.org/) project as the community establishes itself and decides on a code base. Once established, we will transition our development focus to the [Hyperledger](https://www.hyperledger.org/) effort, at which point the code will be maintained for use by IBM.

While we invite [contribution](#contrib) to the IBM Open Blockchain project, we believe that the broader blockchain community focus should be on the shared Linux Foundation [Hyperledger](https://www.hyperledger.org/) project.

## Getting started
Welcome to Open Blockchain (OBC) development!

If you are new to Open Blockchain, and want to learn about our position and the scope of the project, please start with our [whitepaper](whitepaper.md). In addition, we encourage you to review our [glossary](glossary.md) to understand the terminology that we use throughout the website and project.

When you are ready to start using OBC to build applications or to otherwise contribute to the project, we strongly recommend that you read our [protocol specification](protocol-spec.md) for the technical details. Procedurally, we use the agile methodology with a weekly sprint, organized by [issues](https://github.com/openblockchain/obc-peer/issues), so take a look to familiarize yourself with the current work.

## Documentation
In addition to the <b>Getting started</b> documentation, the following topics are also available:
- [Open Blockchain FAQs](https://github.com/openblockchain/obc-docs/tree/master/FAQ)
- [Canonical use cases](https://github.com/openblockchain/obc-docs/blob/master/biz/usecases.md)
- [Development environment set-up](https://github.com/openblockchain/obc-docs/blob/master/dev-setup/devenv.md)
- [Chain code development environment](https://github.com/openblockchain/obc-docs/blob/master/api/SandboxSetup.md)
- [Open Blockchain APIs](https://github.com/openblockchain/obc-docs/blob/master/api/Openchain%20API.md)
- [Open Blockchain network setup](https://github.com/openblockchain/obc-docs/blob/master/dev-setup/devnet-setup.md)
- [Technical implementation details](https://github.com/openblockchain/obc-docs/tree/master/tech)

## License
This software is made available under the [Apache License Version 2.0](LICENSE).

## Setting up the development environment
We have created a development environment for this project such that each contributor has the same set-up and can be productive within a few minutes. Follow the [instructions](dev-setup/devenv.md) to download and start using Open Blockchain.

## Code contributions <a name="contrib"></a>
We are using the [GitHub Flow](https://guides.github.com/introduction/flow/) process to manage code contributions.

Note the following GitHub Flow highlights:

- Anything in the master branch is deployable
- To work on something new, create a descriptively-named branch off of your fork ([more detail on fork](https://help.github.com/articles/syncing-a-fork/))
- Commit to that branch locally, and regularly push your work to the same branch on the server
- When you need feedback or help, or you think the branch is ready for merging,
open a pull request (make sure you have first successfully built and tested with the [Unit and Behave Tests](https://github.com/openblockchain/obc-peer))
- After your pull request has been reviewed and signed off, a committer can merge it into the master branch.

We use the same approach&mdash;the [Developer's Certificate of Origin (DCO)](https://github.com/openblockchain/obc-docs/blob/master/biz/DCO1.1.txt)&mdash;that the Linux&reg; Kernel [community](http://elinux.org/Developer_Certificate_Of_Origin) uses to manage code contributions.
We simply ask that when submitting a pull request, the developer must include a sign-off statement in the pull request description.

Here is an example Signed-off-by line, which indicates that the submitter accepts the DCO:

```
Signed-off-by: John Doe <john.doe@hisdomain.com>
```

## Communication
We use [Slack for communication](https://openchain.slack.com) and
Screenhero or Google Hangouts&trade; for screen sharing between developers. Register with these tools to get connected.

## Coding Golang
- We require a file [header](https://github.com/openblockchain/obc-docs/blob/master/dev-setup/headers.txt) in all source code files. Simply copy and paste the header when you create a new file.
- We code in Go&trade; and strictly follow the [best practices](http://golang.org/doc/effective_go.html)
and will not accept any deviations. You must run the following tools against your Go code and fix all errors and warnings:
	- [golint](https://github.com/golang/lint)
	- [go vet](https://golang.org/cmd/vet/)
