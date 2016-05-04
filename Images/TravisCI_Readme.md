##Continuous Integration Process:#

Continuous Integration is a development practice that requires developers to integrate code into a shared repository. When each time developer checks in code into the repository, it is then verified by an automation build process. This process gives flexibilty for the developer to detect any build issues early in the build life cylce.

**hyperledger** build process is fully automated using **Travis CI** Continuous Integration tool, which helps in building a real time solution for all the code check-ins, perform Unit and FVT tests. This provides instant results to developers / Contributors.

**Master Repository** can be found at [**hyperledger**] (https://github.com/hyperledger/fabric.git).

##Setting up Continuous Integration Process:

- Login in [GIT HUB] (https://github.com) --> Fork and clone the  [**hyperledger**](https://github.com/hyperledger/fabric.git) project into your *GIT HUB* account, if you weren't already. If you have a forked repository in your Git Hub account, please pull **master repository**. So that, .travis.yml (`Configuration file for Travis CI`) file will be copied into your repository.

######Perform **Travis CI** integration in **GIT HUB**:

- Click on **Settings** tab in forked **fabric** Git Hub repository and click on **Webhooks & Services** option. Click on **Add Service** and click on **Travis CI** link in services panel. Provide below details

- User (GitHub UserName)
- Token (Generate token from profile - settings - Personal access token - click on Generate New Token) - Select only public repository and copy and paste it in Token field
- Domain (Enter "notify.travis-ci.org")

- Click on Add Service button

This will enable integration between Travis CI and Git Hub for the selected repository. After successful integration, **Travis CI** service will appear in Services menu.

![Webhook_Travis](./Travis_service.PNG)

###### Sync and Enable fabric repository in Travis:

- Browse [Travis CI](http://travis-ci.org) and click on **Sign in with GitHub** button and provide Git Hub credentials.

- http://travis-ci.org - for Public Repositories. http://travis-ci.com - for Private respositories.

- After login to Travis CI --> Click on *Accounts* link under username (Available at the top of right corner) and click on **Sync account** button. This will sync and display all the repositories available for the logged in user. As a next step user has to flick ON for the required repositories. After successful flick, refresh the Travis home page and you see all the selected repositories available in *My Repositories* section in Travis home page. 
 
- In more options menu, click on **Settings** and enable general settings (**Build only if .travis.yml is present** ,  **Build Pushes** ,  **Limit Current jobs**  , **Build pull requests**) for the selected repository. 

![Settings](./Travis_Settings.PNG) 

- Disable **Build Pull Requests** option if you don't want to trigger automation build deployment for any `Pull Requests`.

**Add Build status markdown link in Readme.md file**

- Copy markdown link from Travis CI home page and place it in Readme.md file of your working branch. Follow [Embedding Status Images](https://docs.travis-ci.com/user/status-images) that helps you to setup the build status in Readme.md file.

Note: Please make sure **.travis.yml** , **foldercopy.sh** and **containerlogs.sh** are present with below modifications in the master branch or the working branch in GIT HUB before performing any ` GIT PUSH ` operations.

- Change notifications section as per user preferences:

Follow [Travis Notification Settings](https://docs.travis-ci.com/user/notifications) to setup notifications in .travis.yml file.

Repository Owner has to provide slack token. Please get in touch with him/her for your Slack Token.

```
notifications:
slack:<Slack account name>:<User Slack Token> ex: slack:openchain:<user slack token>
 on_success: always
 on_failure: always
 email:
    recipients:
      - one@example.com
      - other@example.com
    on_success: [always|never|change] # default: change
    on_failure: [always|never|change] # default: always
  ```

Now you have completed with Travis CI setup process. If you make any changes in your code and push code to remote repository, Travis CI automatically starts the build process and shows you the build results on your notification settings (Slack and on Git Hub Readme.md).

![Build Results](./BuildStatus.PNG )

**Build Process includes below steps:**

1. git clone on updated git repository into Travis cloud environment from github
2. Install all dependency software's and libraries
3. Perform go build
4. Start Peer process
4. Perform unit tests (go tests)
5. Perform Behave test suite (behave tests)
6. Printing failed container log files in travis raw log file.

## More Information:

- Developer can skip the Travis CI build process by providing ` [ci skip] ` in the git commit message.
```
git commit -m "Ignore build process [ci skip]"

```
- How to skip Travis Build execution for PR's:?
  
  - Travis CI checks for latest commit of PR and if the commit message is tagged with [ci skip], Travis CI ignores build process.
  - This will be useful, when you wants to open a pull request early for review but you are not necessarily ready to run the tests right away. Also, you can skip Travis build process for document changes.
  - Right now, Travis only support above method to skip build process.

- What is the slack channel to view build results?
  - We are sending build notifications to hyperledger `#fabric-ci-status` slack channel.
  
- What is build status `errored` means?
  - When go build and Unit tests fails Travis stops the build process and exit from that with build status as `errored`. When unit tests fails Travis CI will not execute behave test scripts. Failed: If Behave test cases fails, Travis continue executing remaining test cases and exit with build status as `failed`.
  
- How to restart build without committing any changes to remote github?

  - Apply `git commit --allow-empty -m "Empty commit" ` and do a git push or click on `Restart Job` (only users with push access to repository can do this) button on Travis CI home page.

- Where can I find Build log files?
  - Click on `RAW log` link on Travis CI home page.

- Where can I find Behave Container log files:
  - Click on each container log link displaying bottom of the RAW log file and it opens in a browser.
