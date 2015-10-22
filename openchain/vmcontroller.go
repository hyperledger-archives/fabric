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

package openchain

import (
	"bytes"
	"fmt"
	"io"

	"github.com/fsouza/go-dockerclient"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
)

//abstract virtual image for supporting arbitrary virual machines
type vm interface {
	build(ctxt context.Context, id string, args []string, reader io.Reader) error
	start(ctxt context.Context, id string, args []string, detach bool, instream io.Reader, outstream io.Writer) error
	stop(ctxt context.Context, id string, timeout uint) error
}

//dockerVM is a vm. It is identified by an image id
type dockerVM struct {
	id string
}

//create a docker client given endpoint to communicate with docker host
func (vm *dockerVM) newClient() (*docker.Client, error) {
	//QQ: is this ok using config properties here so deep ? ie, should we read these in main and stow them away ?
	endpoint := viper.GetString("vm.endpoint")
	fmt.Printf("Creating dockerVM with endpoint: %s\n", endpoint)
	client, err := docker.NewClient(endpoint)
	if err != nil {
		return nil, err
	}
	return client, nil
}

//for docker inputbuf is tar reader ready for use by docker.Client
//the stream from end client to peer could directly be this tar stream
//talk to docker daemon using docker Client and build the image
func (vm *dockerVM) build(ctxt context.Context, id string, args []string, reader io.Reader) error {
	outputbuf := bytes.NewBuffer(nil)
	opts := docker.BuildImageOptions{
		Name:         id,
		Pull:         true,
		InputStream:  reader,
		OutputStream: outputbuf,
	}
	client, err := vm.newClient()
	switch err {
	case nil:
		if err = client.BuildImage(opts); err != nil {
			fmt.Printf("Error building Peer container: %s", err)
			return err
		}
	default:
		return fmt.Errorf("Error creating docker client: %s", err)
	}
	config := docker.Config{Cmd: args, Image: id}
	copts := docker.CreateContainerOptions{Name: id, Config: &config}
	_, err = client.CreateContainer(copts)
	if err != nil {
		return err
	}
	return nil
}

func (vm *dockerVM) start(ctxt context.Context, id string, args []string, detach bool, instream io.Reader, outstream io.Writer) error {
	client, err := vm.newClient()
	if err != nil {
		fmt.Printf("start - cannot create client %s\n", err)
		return err
	}
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

func (vm *dockerVM) stop(ctxt context.Context, id string, timeout uint) error {
	client, err := vm.newClient()
	if err != nil {
		fmt.Printf("start - cannot create client %s\n", err)
		return err
	}
	err = client.StopContainer(id, timeout)
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

//VMController - manages VMs
//   . abstract construction of different types of VMs (we only care about Docker for now)
//   . manage lifecycle of VM (start with build, start, stop ...
//     eventually probably need fine grained management)
type VMController struct{}

//singleton...acess through NewVMController
var vmcontroller *VMController

//NewVMController - creates/returns singleton
func init() {
	vmcontroller = new(VMController)
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

//VMCReqIntf - all requests should implement this interface.
//The context should be passed and tested at each layer till we stop
//note that we'd stop on the first method on the stack that does not
//take context
type VMCReqIntf interface {
	do(ctxt context.Context, v vm) VMCResp
}

//VMCResp - response from requests. resp field is a anon interface.
//It can hold any response. err should be tested first
type VMCResp struct {
	Err  error
	Resp interface{}
}

//CreateImageReq - properties for creating an container image
type CreateImageReq struct {
	Id     string
	Reader io.Reader
	Args   []string
}

func (bp CreateImageReq) do(ctxt context.Context, v vm) VMCResp {
	var resp VMCResp
	if err := v.build(ctxt, bp.Id, bp.Args, bp.Reader); err != nil {
		resp = VMCResp{Err: err}
	} else {
		resp = VMCResp{}
	}

	return resp
}

//StartImageReq - properties for starting a container.
type StartImageReq struct {
	Id        string
	Args      []string
	Detach    bool
	Instream  io.Reader
	Outstream io.Writer
}

func (si StartImageReq) do(ctxt context.Context, v vm) VMCResp {
	var resp VMCResp
	if err := v.start(ctxt, si.Id, si.Args, si.Detach, si.Instream, si.Outstream); err != nil {
		resp = VMCResp{Err: err}
	} else {
		resp = VMCResp{}
	}

	return resp
}

//StopImageReq - properties for stopping a container.
type StopImageReq struct {
	Id      string
	Timeout uint
}

func (si StopImageReq) do(ctxt context.Context, v vm) VMCResp {
	var resp VMCResp
	if err := v.stop(ctxt, si.Id, si.Timeout); err != nil {
		resp = VMCResp{Err: err}
	} else {
		resp = VMCResp{}
	}

	return resp
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
		resp = req.do(ctxt, v)
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
