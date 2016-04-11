/*
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
*/

package dockercontroller

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	cutil "github.com/hyperledger/fabric/core/container/util"
	"github.com/fsouza/go-dockerclient"
	"golang.org/x/net/context"
)

//DockerVM is a vm. It is identified by an image id
type DockerVM struct {
	id string
}

func (vm *DockerVM) createContainer(ctxt context.Context, client *docker.Client, imageID string, containerID string, args []string, env []string, attachstdin bool, attachstdout bool) error {
	config := docker.Config{Cmd: args, Image: imageID, Env: env, AttachStdin: attachstdin, AttachStdout: attachstdout}
	copts := docker.CreateContainerOptions{Name: containerID, Config: &config}
	//TODO UNCOMMENT vmLogger.Debug("Create container: %s", containerID)
	_, err := client.CreateContainer(copts)
	if err != nil {
		return err
	}
	//TODO UNCOMMENT vmLogger.Debug("Created container: %s", imageID)
	return nil
}

//for docker inputbuf is tar reader ready for use by docker.Client
//the stream from end client to peer could directly be this tar stream
//talk to docker daemon using docker Client and build the image
func (vm *DockerVM) Deploy(ctxt context.Context, id string, args []string, env []string, attachstdin bool, attachstdout bool, reader io.Reader) error {
	outputbuf := bytes.NewBuffer(nil)
	opts := docker.BuildImageOptions{
		Name:         id,
		Pull:         false,
		InputStream:  reader,
		OutputStream: outputbuf,
	}
	client, err := cutil.NewDockerClient()
	switch err {
	case nil:
		if err = client.BuildImage(opts); err != nil {
			//TODO UNCOMMENT vmLogger.Error(fmt.Sprintf("Error building Peer container: %s", err))
			return err
		}
		//TODO UNCOMMENT vmLogger.Debug("Created image: %s", id)
	default:
		return fmt.Errorf("Error creating docker client: %s", err)
	}
	return nil
}

func (vm *DockerVM) Start(ctxt context.Context, imageID string, args []string, env []string, attachstdin bool, attachstdout bool) error {
	client, err := cutil.NewDockerClient()
	if err != nil {
		//TODO UNCOMMENT vmLogger.Debug("start - cannot create client %s", err)
		return err
	}

	containerID := strings.Replace(imageID, ":", "_", -1)

	//stop,force remove if necessary
	//TODO UNCOMMENT vmLogger.Debug("Cleanup container %s", containerID)
	vm.stopInternal(ctxt, client, containerID, 0, false, false)

	//TODO UNCOMMENT vmLogger.Debug("Start container %s", containerID)
	err = vm.createContainer(ctxt, client, imageID, containerID, args, env, attachstdin, attachstdout)
	if err != nil {
		//TODO UNCOMMENT vmLogger.Error(fmt.Sprintf("start-could not recreate container %s", err))
		return err
	}
	err = client.StartContainer(containerID, &docker.HostConfig{NetworkMode: "host"})
	if err != nil {
		//TODO UNCOMMENT vmLogger.Error(fmt.Sprintf("start-could not start container %s", err))
		return err
	}

	//TODO UNCOMMENT vmLogger.Debug("Started container %s", containerID)
	return nil
}

func (vm *DockerVM) Stop(ctxt context.Context, id string, timeout uint, dontkill bool, dontremove bool) error {
	client, err := cutil.NewDockerClient()
	if err != nil {
		//TODO UNCOMMENT vmLogger.Debug("start - cannot create client %s", err)
		return err
	}
	id = strings.Replace(id, ":", "_", -1)

	err = vm.stopInternal(ctxt, client, id, timeout, dontkill, dontremove)

	return err
}

func (vm *DockerVM) stopInternal(ctxt context.Context, client *docker.Client, id string, timeout uint, dontkill bool, dontremove bool) error {
	err := client.StopContainer(id, timeout)
	if err != nil {
		//TODO UNCOMMENT vmLogger.Debug("Stop container %s(%s)", id, err)
	} else {
		//TODO UNCOMMENT vmLogger.Debug("Stopped container %s", id)
	}
	if !dontkill {
		err = client.KillContainer(docker.KillContainerOptions{ID: id})
		if err != nil {
			//TODO UNCOMMENT vmLogger.Debug("Kill container %s (%s)", id, err)
		} else {
			//TODO UNCOMMENT vmLogger.Debug("Killed container %s", id)
		}
	}
	if !dontremove {
		err = client.RemoveContainer(docker.RemoveContainerOptions{ID: id, Force: true})
		if err != nil {
			//TODO UNCOMMENT vmLogger.Debug("Remove container %s (%s)", id, err)
		} else {
			//TODO UNCOMMENT vmLogger.Debug("Removed container %s", id)
		}
	}
	return err
}

