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
	"errors"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
	"io"
)

//abstract virtual image for supporting arbitrary virual machines
type vm interface {
	build(ctxt context.Context, id string, reader io.Reader) error
	start(ctxt context.Context) error
	stop(ctxt context.Context) error
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
func (vm *dockerVM) build(ctxt context.Context, id string, reader io.Reader) error {
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
			return errors.New(fmt.Sprintf("Error building Peer container: %s", err))
		}
	default:
		return errors.New(fmt.Sprintf("Error creating docker client: %s", err))
	}
	fmt.Sprintf("built image for %s successfully", id)
	return nil
}

func (vm *dockerVM) start(ctxt context.Context) error {
	return nil
}

func (vm *dockerVM) stop(ctxt context.Context) error {
	return nil
}

const (
	DOCKER = "Docker"
)

type image struct {
	id   string
	args []string
	v    vm
}

//VMController's role
//   . abstract construction of different types of VMs (we only care about Docker for now)
//   . maintain an id->vm map for look ups (TODO - think about versions of same "id")
//   . manage lifecycle of VM (start with build, start, stop ... eventually probably need fine grained management)
type VMController struct {
	images map[string]*image
}

var vmcontroller *VMController = nil

//singletone constructor
func NewVMController() *VMController {
	if vmcontroller == nil {
		vmcontroller = new(VMController)
		//TODO initialize VMController
		vmcontroller.images = map[string]*image{}
	}
	return vmcontroller
}

func (vmc *VMController) newVM(typ string) vm {
	var (
		v vm = nil
	)

	switch typ {
	case DOCKER:
		v = &dockerVM{}
	case "":
		v = &dockerVM{}
	}
	return v
}

type VMCReqIntf interface {
	do(ctxt context.Context, v vm) interface{}
}

//response from requests. resp field is a anon interface. It can hold any response
//err should be tested first
type VMCResp struct {
	err  error
	resp interface{}
}

type CreateImageReq struct {
	id     string
	reader io.Reader
	args   []string
}

type CreateImageResp struct {
	success bool
}

func (bp CreateImageReq) do(ctxt context.Context, v vm) interface{} {
	//image was constructed and exists
	if vmcontroller.images[bp.id] != nil {
		return VMCResp{}
	}

	var resp VMCResp
	if err := v.build(ctxt, bp.id, bp.reader); err != nil {
		resp = VMCResp{err: err}
	} else {
		vmargs := make([]string, len(bp.args))
		copy(vmargs, bp.args)
		vmcontroller.images[bp.id] = &image{id: bp.id, args: vmargs, v: v}
		resp = VMCResp{}
	}

	return resp
}

//use Process as follows
//   . construct a context
//   . construct req of the right type (e.g., CreateImageReq)
//   . call it in a go routine
//   . process response in the go routing
//context can be cancelled. Process will try to cancel calling functions if it can
//For instance docker clients api's such as BuildImage are not cancelable.
//In all cases Process will wait for the called go routine to return
func (vmc *VMController) Process(ctxt context.Context, vmtype string, req VMCReqIntf) (interface{}, error) {
	v := vmc.newVM(vmtype)

	if v == nil {
		return nil, errors.New(fmt.Sprintf("Unknown VM type %s", vmtype))
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
