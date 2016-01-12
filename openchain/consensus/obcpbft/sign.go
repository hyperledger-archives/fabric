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

package obcpbft

import (
	"fmt"
	"reflect"

	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/util"
)

type signable interface {
	getSignature() []byte
	setSignature(s []byte)
	getID() []byte
	setID(id []byte)
	serialize() ([]byte, error)
}

func (instance *pbftCore) sign(s signable) error {
	s.setSignature(nil)
	// TODO once the crypto package is integrated
	// cryptoID, _, _ := instance.cpi.GetReplicaID()
	// s.setID(cryptoID)
	id := []byte("XXX ID")
	s.setID(id)
	raw, err := s.serialize()
	if err != nil {
		return err
	}
	// s.setSignature(instance.cpi.Sign(raw))
	s.setSignature(util.ComputeCryptoHash(append(id, raw...)))
	return nil
}

func (instance *pbftCore) verify(s signable) error {
	origSig := s.getSignature()
	s.setSignature(nil)
	raw, err := s.serialize()
	s.setSignature(origSig)
	if err != nil {
		return err
	}
	id := s.getID()
	// XXX check that s.Id() is a valid replica
	// instance.cpi.Verify(s.Id(), origSig, raw)
	if !reflect.DeepEqual(util.ComputeCryptoHash(append(id, raw...)), origSig) {
		return fmt.Errorf("invalid signature")
	}
	return nil
}

func (vc *ViewChange) getSignature() []byte {
	return vc.Signature
}

func (vc *ViewChange) setSignature(sig []byte) {
	vc.Signature = sig
}

func (vc *ViewChange) getID() []byte {
	return []byte("XXX ID")
}

func (vc *ViewChange) setID(id []byte) {
	// XXX set id
}

func (vc *ViewChange) serialize() ([]byte, error) {
	return proto.Marshal(vc)
}

func (v *Verify) getSignature() []byte {
	return v.Signature
}

func (v *Verify) setSignature(sig []byte) {
	v.Signature = sig
}

func (v *Verify) getID() []byte {
	return []byte("XXX ID")
}

func (v *Verify) setID(id []byte) {
	// XXX set ID
}

func (v *Verify) serialize() ([]byte, error) {
	return proto.Marshal(v)
}

func (msg *VerifySet) getSignature() []byte {
	return msg.Signature
}

func (msg *VerifySet) setSignature(sig []byte) {
	msg.Signature = sig
}

func (msg *VerifySet) getID() []byte {
	return []byte("XXX ID")
}

func (msg *VerifySet) setID(id []byte) {
	// XXX set ID
}

func (msg *VerifySet) serialize() ([]byte, error) {
	return proto.Marshal(msg)
}

func (msg *Flush) getSignature() []byte {
	return msg.Signature
}

func (msg *Flush) setSignature(sig []byte) {
	msg.Signature = sig
}

func (msg *Flush) getID() []byte {
	return []byte("XXX ID")
}

func (msg *Flush) setID(id []byte) {
	// XXX set ID
}

func (msg *Flush) serialize() ([]byte, error) {
	return proto.Marshal(msg)
}
