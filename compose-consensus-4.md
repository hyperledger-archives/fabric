The set of *compose-consensus-4* files can be used with docker-compose to create a network of validating peers primarily for use in testing and development of consensus protocols.

6 Docker containers are created and linked such that the containers can communicate with each other using hostnames. Currently, the entire network runs on one physical machine **inside a Vagrant box**. The containers created are:
* obcca (obcpeer_obcca_1) - the obc-ca server
* cli (obcpeer_cli_1) - you can attach to this container and issue CLI or REST API commands
* vp0 (obcpeer_vp0_1), vp1 (obcpeer_vp1_1), vp2 (obcpeer_vp2_1), vp3 (obcpeer_vp3_1) - 4 validating peers all running the same consensus protocol ( this is customizable by specifying different docker-compose files. See below. )
The first name is the **hostname** of the container. The name in parenthesis is the **container name** automatically assigned by Docker.

**Note:** When you deploy a chaincode, each peer will create another Docker container.

Unless otherwise noted, you are at $GOPATH/src/github.com/openchain/obc-peer

### Issues
This is what is not working right
* cannot create a directory. Specifically when I run **obcpeer login xxx** . It says *cannot create /var/openchain/production/client*. I get around it by creating the directory manually and redoing the command. You might not see this error, especially if you've run obc-peer before. This will be fixed in a separate pull request.

### The *infiniteloop.sh* shell script
We use the cli container as the spot to run the client and issue CLI or REST API calls. In order for the container to stay up until we connect to it, we need to have it start and wait. *infiniteloop.sh* is just an infinite echo/sleep loop that keeps the container up until we can do a `Docker exec` to it.

On some operating systems, you'll need to set execute permission on the script file.

### Manual Configuration
 1. When you login to the client, use one of the _test_user**x**_ IDs defined in _obc-ca/obcca.yaml_. You can find more details about IDs and roles in the [SandboxSetup](https://github.com/openblockchain/obc-docs/blob/master/api/SandboxSetup.md) document.


 ### Create Docker images for the obc-peer server and the obc-ca server.
From $GOPATH/src/github.com/openchain/obc-peer , create the obc-peer Docker image by running command
```
docker build -t openchain-peer .
```
then create the obc-ca Docker image by running command
```
docker build -t obcca -f obc-ca/Dockerfile .
```
or you can follow the instructions in the  [README](https://github.com/openblockchain/obc-peer/blob/master/README.md) document.

You can verify that the images have been created and are available by running command
```
docker images
```

### Run docker-compose to create the network and start all the containers
Rename or backup file *docker-compose.yml* then copy or rename file *compose-consensus-4.yml* into file *docker-compose.yml*
```
cp compose-consensus-4.yml docker-compose.yml
```
(This is necessary because Docker requires the first file to be docker-compose.yml and we are not using the original docker-compose.yml file).

Then start up the network by running command
```
docker-compose -f docker-compose.yml -f compose-consensus-4-links.yml -f compose-consensus-4-sieve.yml up --force-recreate -d
```
if you want to run pbft sieve, batch or classic, specify the appropriate *compose-consensus-4-(batch or classic or sieve).yml* file.

The *--force-recreate* option tells Docker to recreate all the containers.
The *-d* option runs the containers as background processes. If you do not set this option, the log output of all containers go to your terminal.

### Connect to the cli container and run your tests
From another Vagrant terminal, run this command
```
docker exec -it obcpeer_cli_1 bash
```
You'll get a bash prompt inside the container. From here, you can issue any CLI or REST API command. When you are done, run command
```
exit
```
which returns out out of the obcpeer_cli_1 container. Note that the container is still running.

### More useful Docker commands
* show all containers in the docker-compose group
```
docker-compose ps
```
* stop all the docker-compose containers
```
docker-compose stop
```
* delete all the docker-compose containers
```
docker-compose rm
```
* attach to the console of a container
```
docker attach --sig-proxy=false container_name
```
this is the console for the output of the command the container started with, e.g. where obc-peer outputs its log records.
Type ^P^Q to exit ( this works intermittently ). If --sig-proxy=true, the container stops when you exit)
* get the log of a container
```
docker logs container_name
```
best to redirect to a file. Otherwise, it just prints to your stdout.
* show all containers in the system
```
docker ps -a
```
the *-a* option includes the inactive containers
* list all Docker images
```
docker images
```
* delete a Docker image
```
docker rmi imagename1 imagename2 ...
```
