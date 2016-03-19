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

package container

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/fsouza/go-dockerclient"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
)

//abstract virtual image for supporting arbitrary virual machines
type vm interface {
	build(ctxt context.Context, id string, args []string, env []string, attachstdin bool, attachstdout bool, reader io.Reader) error
	start(ctxt context.Context, id string, args []string, env []string, attachstdin bool, attachstdout bool) error
	stop(ctxt context.Context, id string, timeout uint, dontkill bool, dontremove bool) error
}

//dockerVM is a vm. It is identified by an image id
type dockerVM struct {
	id string
}

//create a docker client given endpoint to communicate with docker host
func (vm *dockerVM) newClient() (*docker.Client, error) {
	return newDockerClient()
}

func (vm *dockerVM) createContainer(ctxt context.Context, client *docker.Client, imageID string, containerID string, args []string, env []string, attachstdin bool, attachstdout bool) error {
	config := docker.Config{Cmd: args, Image: imageID, Env: env, AttachStdin: attachstdin, AttachStdout: attachstdout}
	copts := docker.CreateContainerOptions{Name: containerID, Config: &config}
	vmLogger.Debug("Create container: %s", containerID)
	_, err := client.CreateContainer(copts)
	if err != nil {
		return err
	}
	vmLogger.Debug("Created container: %s", imageID)
	return nil
}

//for docker inputbuf is tar reader ready for use by docker.Client
//the stream from end client to peer could directly be this tar stream
//talk to docker daemon using docker Client and build the image
func (vm *dockerVM) build(ctxt context.Context, id string, args []string, env []string, attachstdin bool, attachstdout bool, reader io.Reader) error {
	outputbuf := bytes.NewBuffer(nil)
	opts := docker.BuildImageOptions{
		Name:         id,
		Pull:         false,
		InputStream:  reader,
		OutputStream: outputbuf,
	}
	client, err := vm.newClient()
	switch err {
	case nil:
		if err = client.BuildImage(opts); err != nil {
			vmLogger.Error(fmt.Sprintf("Error building Peer container: %s", err))
			return err
		}
		vmLogger.Debug("Created image: %s", id)
	default:
		return fmt.Errorf("Error creating docker client: %s", err)
	}
	return nil
}

func (vm *dockerVM) start(ctxt context.Context, imageID string, args []string, env []string, attachstdin bool, attachstdout bool) error {
	client, err := vm.newClient()
	if err != nil {
		vmLogger.Debug("start - cannot create client %s", err)
		return err
	}

	containerID := strings.Replace(imageID, ":", "_", -1)

	//stop,force remove if necessary
	vmLogger.Debug("Cleanup container %s", containerID)
	vm.stopInternal(ctxt, client, containerID, 0, false, false)

	vmLogger.Debug("Start container %s", containerID)
	err = vm.createContainer(ctxt, client, imageID, containerID, args, env, attachstdin, attachstdout)
	if err != nil {
		vmLogger.Error(fmt.Sprintf("start-could not recreate container %s", err))
		return err
	}
	err = client.StartContainer(containerID, &docker.HostConfig{NetworkMode: "host"})
	if err != nil {
		vmLogger.Error(fmt.Sprintf("start-could not start container %s", err))
		return err
	}

	vmLogger.Debug("Started container %s", containerID)
	return nil
}

func (vm *dockerVM) stop(ctxt context.Context, id string, timeout uint, dontkill bool, dontremove bool) error {
	client, err := vm.newClient()
	if err != nil {
		vmLogger.Debug("start - cannot create client %s", err)
		return err
	}
	id = strings.Replace(id, ":", "_", -1)

	err = vm.stopInternal(ctxt, client, id, timeout, dontkill, dontremove)

	return err
}

func (vm *dockerVM) stopInternal(ctxt context.Context, client *docker.Client, id string, timeout uint, dontkill bool, dontremove bool) error {
	err := client.StopContainer(id, timeout)
	if err != nil {
		vmLogger.Debug("Stop container %s(%s)", id, err)
	} else {
		vmLogger.Debug("Stopped container %s", id)
	}
	if !dontkill {
		err = client.KillContainer(docker.KillContainerOptions{ID: id})
		if err != nil {
			vmLogger.Debug("Kill container %s (%s)", id, err)
		} else {
			vmLogger.Debug("Killed container %s", id)
		}
	}
	if !dontremove {
		err = client.RemoveContainer(docker.RemoveContainerOptions{ID: id, Force: true})
		if err != nil {
			vmLogger.Debug("Remove container %s (%s)", id, err)
		} else {
			vmLogger.Debug("Removed container %s", id)
		}
	}
	return err
}

//constants for supported containers
const (
	DOCKER = "Docker"
)

type image struct {
	id   string
	args []string
	v    vm
}

type refCountedLock struct {
	refCount int
	lock     *sync.RWMutex
}

//VMController - manages VMs
//   . abstract construction of different types of VMs (we only care about Docker for now)
//   . manage lifecycle of VM (start with build, start, stop ...
//     eventually probably need fine grained management)
type VMController struct {
	sync.RWMutex
	// Handlers for each chaincode
	containerLocks map[string]*refCountedLock
}

//singleton...acess through NewVMController
var vmcontroller *VMController

//NewVMController - creates/returns singleton
func init() {
	vmcontroller = new(VMController)
	vmcontroller.containerLocks = make(map[string]*refCountedLock)
}

func (vmc *VMController) newVM(typ string) vm {
	var (
		v vm
	)

	switch typ {
	case DOCKER:
		v = &dockerVM{}
	case "":
		v = &dockerVM{}
	}
	return v
}

func (vmc *VMController) lockContainer(id string) {
	//get the container lock under global lock
	vmcontroller.Lock()
	var refLck *refCountedLock
	var ok bool
	if refLck, ok = vmcontroller.containerLocks[id]; !ok {
		refLck = &refCountedLock{refCount: 1, lock: &sync.RWMutex{}}
		vmcontroller.containerLocks[id] = refLck
	} else {
		refLck.refCount++
		vmLogger.Debug("refcount %d (%s)", refLck.refCount, id)
	}
	vmcontroller.Unlock()
	vmLogger.Debug("waiting for container(%s) lock", id)
	refLck.lock.Lock()
	vmLogger.Debug("got container (%s) lock", id)
}

func (vmc *VMController) unlockContainer(id string) {
	vmcontroller.Lock()
	if refLck, ok := vmcontroller.containerLocks[id]; ok {
		if refLck.refCount <= 0 {
			panic("refcnt <= 0")
		}
		refLck.lock.Unlock()
		if refLck.refCount--; refLck.refCount == 0 {
			vmLogger.Debug("container lock deleted(%s)", id)
			delete(vmcontroller.containerLocks, id)
		}
	} else {
		vmLogger.Debug("no lock to unlock(%s)!!", id)
	}
	vmcontroller.Unlock()
}

//VMCReqIntf - all requests should implement this interface.
//The context should be passed and tested at each layer till we stop
//note that we'd stop on the first method on the stack that does not
//take context
type VMCReqIntf interface {
	do(ctxt context.Context, v vm) VMCResp
	getID() string
}

//VMCResp - response from requests. resp field is a anon interface.
//It can hold any response. err should be tested first
type VMCResp struct {
	Err  error
	Resp interface{}
}

//CreateImageReq - properties for creating an container image
type CreateImageReq struct {
	ID           string
	Reader       io.Reader
	AttachStdin  bool
	AttachStdout bool
	Args         []string
	Env          []string
}

func (bp CreateImageReq) do(ctxt context.Context, v vm) VMCResp {
	var resp VMCResp
	if err := v.build(ctxt, bp.ID, bp.Args, bp.Env, bp.AttachStdin, bp.AttachStdout, bp.Reader); err != nil {
		resp = VMCResp{Err: err}
	} else {
		resp = VMCResp{}
	}

	return resp
}

func (bp CreateImageReq) getID() string {
	return bp.ID
}

//StartImageReq - properties for starting a container.
type StartImageReq struct {
	ID           string
	Args         []string
	Env          []string
	AttachStdin  bool
	AttachStdout bool
}

func (si StartImageReq) do(ctxt context.Context, v vm) VMCResp {
	var resp VMCResp
	if err := v.start(ctxt, si.ID, si.Args, si.Env, si.AttachStdin, si.AttachStdout); err != nil {
		resp = VMCResp{Err: err}
	} else {
		resp = VMCResp{}
	}

	return resp
}

func (si StartImageReq) getID() string {
	return si.ID
}

//StopImageReq - properties for stopping a container.
type StopImageReq struct {
	ID      string
	Timeout uint
	//by default we will kill the container after stopping
	Dontkill bool
	//by default we will remove the container after killing
	Dontremove bool
}

func (si StopImageReq) do(ctxt context.Context, v vm) VMCResp {
	var resp VMCResp
	if err := v.stop(ctxt, si.ID, si.Timeout, si.Dontkill, si.Dontremove); err != nil {
		resp = VMCResp{Err: err}
	} else {
		resp = VMCResp{}
	}

	return resp
}

func (si StopImageReq) getID() string {
	return si.ID
}

//VMCProcess should be used as follows
//   . construct a context
//   . construct req of the right type (e.g., CreateImageReq)
//   . call it in a go routine
//   . process response in the go routing
//context can be cancelled. VMCProcess will try to cancel calling functions if it can
//For instance docker clients api's such as BuildImage are not cancelable.
//In all cases VMCProcess will wait for the called go routine to return
func VMCProcess(ctxt context.Context, vmtype string, req VMCReqIntf) (interface{}, error) {
	v := vmcontroller.newVM(vmtype)

	if v == nil {
		return nil, fmt.Errorf("Unknown VM type %s", vmtype)
	}

	c := make(chan struct{})
	var resp interface{}
	go func() {
		defer close(c)
		id := req.getID()
		vmcontroller.lockContainer(id)
		resp = req.do(ctxt, v)
		vmcontroller.unlockContainer(id)
	}()

	select {
	case <-c:
		return resp, nil
	case <-ctxt.Done():
		//TODO cancel req.do ... (needed) ?
		<-c
		return nil, ctxt.Err()
	}
}

//GetVMFromName generates the docker image from peer information given the hashcode. This is needed to
//keep image name's unique in a single host, multi-peer environment (such as a development environment)
func GetVMFromName(name string) string {
	vmName := fmt.Sprintf("%s-%s-%s", viper.GetString("peer.networkId"), viper.GetString("peer.id"), name)
	return vmName
}

/*******************
 * OLD ... leavethis here as sample for "client.CreateExec" in case we need it at some point
func (vm *dockerVM) start(ctxt context.Context, id string, args []string, detach bool, instream io.Reader, outstream io.Writer) error {
	client, err := vm.newClient()
	if err != nil {
		fmt.Printf("start - cannot create client %s\n", err)
		return err
	}
	id = strings.Replace(id, ":", "_", -1)
	fmt.Printf("starting container %s\n", id)
	econfig := docker.CreateExecOptions{
		Container:    id,
		Cmd:          args,
		AttachStdout: true,
	}
	execObj, err := client.CreateExec(econfig)
	if err != nil {
		//perhaps container not started
		err = client.StartContainer(id, &docker.HostConfig{})
		if err != nil {
			fmt.Printf("start-could not start container %s\n", err)
			return err
		}
		execObj, err = client.CreateExec(econfig)
	}

	if err != nil {
		fmt.Printf("start-could not create exec %s\n", err)
		return err
	}
	sconfig := docker.StartExecOptions{
		Detach:       detach,
		InputStream:  instream,
		OutputStream: outstream,
	}
	err = client.StartExec(execObj.ID, sconfig)
	if err != nil {
		fmt.Printf("start-could not start exec %s\n", err)
		return err
	}
	fmt.Printf("start-started and execed container for %s\n", id)
	return nil
}
****************************/
