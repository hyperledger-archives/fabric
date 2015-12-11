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

package pbft

import (
	"fmt"
	"reflect"

	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/util"
)

type signable interface {
	GetSignature() []byte
	SetSignature(s []byte)
	GetId() []byte
	SetId(id []byte)
	Serialize() ([]byte, error)
}

func (instance *Plugin) sign(s signable) error {
	s.SetSignature(nil)
	// s.SetId(instance.cpi.GetId())
	id := []byte("XXX ID")
	s.SetId(id)
	raw, err := s.Serialize()
	if err != nil {
		return err
	}
	// s.SetSignature(instance.cpi.Sign(raw))
	s.SetSignature(util.ComputeCryptoHash(append(id, raw...)))
	return nil
}

func (instance *Plugin) verify(s signable) error {
	origsig := s.GetSignature()
	s.SetSignature(nil)
	raw, err := s.Serialize()
	s.SetSignature(origsig)
	if err != nil {
		return err
	}
	id := s.GetId()
	// XXX check that s.Id() is a valid replica
	// instance.cpi.Verify(s.Id(), origsig, raw)
	if !reflect.DeepEqual(util.ComputeCryptoHash(append(id, raw...)), origsig) {
		return fmt.Errorf("invalid signature")
	}
	return nil
}

func (vc *ViewChange) GetSignature() []byte {
	return vc.Signature
}

func (vc *ViewChange) SetSignature(sig []byte) {
	vc.Signature = sig
}

func (vc *ViewChange) GetId() []byte {
	return []byte("XXX ID")
}

func (vc *ViewChange) SetId(id []byte) {
	// XXX set id
}

func (vc *ViewChange) Serialize() ([]byte, error) {
	return proto.Marshal(vc)
}

func (vc *Verify) GetSignature() []byte {
	return vc.Signature
}

func (vc *Verify) SetSignature(sig []byte) {
	vc.Signature = sig
}

func (vc *Verify) GetId() []byte {
	return []byte("XXX ID")
}

func (vc *Verify) SetId(id []byte) {
	// XXX set id
}

func (vc *Verify) Serialize() ([]byte, error) {
	return proto.Marshal(vc)
}
