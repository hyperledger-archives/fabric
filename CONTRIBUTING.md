### Welcome

We welcome contributions to the Hyperledger Project in many forms, and there's always plenty to do!

First things first, please review the Hyperledger Project's [Code of Conduct](https://github.com/hyperledger/hyperledger/wiki/Hyperledger-Project-Code-of-Conduct) before participating. It is important that we keep things civil.

### Requirements and Use Cases
We have a [Requirements WG](https://github.com/hyperledger/hyperledger/wiki/Requirements-WG) that is documenting use cases and from those use cases deriving requirements. If you are interested in contributing to this effort, please feel free to join the discussion in [slack](https://hyperledgerproject.slack.com/messages/requirements/).

### Reporting bugs
If you are a user and you find a bug, please submit an [issue](https://github.com/hyperledger/fabric/issues). Please try to provide sufficient information for someone else to reproduce the issue. One of the project's maintainers should respond to your issue within 24 hours. If not, please bump the issue and request that it be reviewed.

### Fixing issues and working stories
Review the [issues list](https://github.com/hyperledger/fabric/issues) and find something that interests you. It is wise to start with something relatively straight forward and achievable.

The first step is to create a fork of the [project](http://github.com/hyperledger/fabric) on GitHub and then clone it locally:

```text
$ git clone git@github.com:username/fabric.git
$ cd fabric
$ git remote add upstream git://github.com/hyperledger/fabric.git
```

We are using the [GitHub Flow](https://guides.github.com/introduction/flow/) process to manage code contributions. If you are unfamiliar, please review that link before proceeding.

Note the following GitHub Flow highlights:

- Anything in the master branch is deployable
- To work on something new, create a descriptively-named branch off of your fork ([more detail here](https://help.github.com/articles/syncing-a-fork/))
- Commit to that branch locally, and regularly push your work to the same branch on the server
- When you need feedback or help, or you think the branch is ready for merging,
open a pull request (make sure you have first successfully built and tested with the [Unit and Behave Tests](https://github.com/hyperledger/fabric#3-test))
- Did we mention tests? All code changes should be accompanied by new or modified tests.
- After your pull request has been reviewed and signed off, a committer can merge it into the master branch.

### Becoming a maintainer
This project is managed under open governance model as described in our  [charter](https://www.hyperledger.org/about/charter). Projects or sub-projects will be lead by a set of maintainers. New projects can designate an initial set of maintainers that will be approved by the Technical Steering Committee when the project is first approved. The project's maintainers will, from time-to-time, consider adding a new maintainer. An existing maintainer will post a pull request to the [MAINTAINERS.txt](MAINTAINERS.txt) file. If a majority of the maintainers concur in the comments, the pull request is then merged and the individual becomes a maintainer.

### Legal stuff
We have tried to make it as easy as possible to make contributions. This applies to how we handle the legal aspects of contribution. We use the same approach&mdash;the [Developer's Certificate of Origin (DCO)](docs/biz/DCO1.1.txt)&mdash;that the Linux&reg; Kernel [community](http://elinux.org/Developer_Certificate_Of_Origin) uses to manage code contributions.
We simply ask that when submitting a pull request, the developer must include a sign-off statement in the pull request description.

Here is an example Signed-off-by line, which indicates that the submitter accepts the DCO:

```
Signed-off-by: John Doe <john.doe@hisdomain.com>
```
