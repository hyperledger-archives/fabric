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

func (instance *Plugin) sendViewChange() error {
	instance.view += 1
	instance.activeView = false

	// P set: requests that have prepared here
	//
	// "<n,d,v> has a prepared certificate, and no request
	// prepared in a later view with the same number"

	for idx, cert := range instance.certStore {
		if cert.prePrepare == nil {
			continue
		}

		digest := cert.prePrepare.RequestDigest
		if !instance.prepared(digest, idx.v, idx.n) {
			continue
		}

		if p, ok := instance.pset[idx.n]; ok && p.View > idx.v {
			continue
		}

		instance.pset[idx.n] = &ViewChange_PQ{
			SequenceNumber: idx.n,
			Digest:         digest,
			View:           idx.v,
		}
	}

	// Q set: requests that have pre-prepared here (pre-prepare or
	// prepare sent)
	//
	// "<n,d,v>: requests that pre-prepared here, and did not
	// pre-prepare in a later view with the same number"

	for idx, cert := range instance.certStore {
		if cert.prePrepare == nil {
			continue
		}

		digest := cert.prePrepare.RequestDigest
		if !instance.prePrepared(digest, idx.v, idx.n) {
			continue
		}

		qi := qidx{digest, idx.n}
		if q, ok := instance.qset[qi]; ok && q.View > idx.v {
			continue
		}

		instance.qset[qi] = &ViewChange_PQ{
			SequenceNumber: idx.n,
			Digest:         digest,
			View:           idx.v,
		}
	}

	vc := &ViewChange{
		View:      instance.view,
		H:         instance.h,
		ReplicaId: instance.id,
	}

	for _, chkpt := range instance.chkpts {
		vc.Cset = append(vc.Cset, &ViewChange_C{
			SequenceNumber: chkpt.n,
			Digest:         chkpt.state,
		})
	}

	for _, p := range instance.pset {
		vc.Pset = append(vc.Pset, p)
	}

	for _, q := range instance.qset {
		vc.Qset = append(vc.Qset, q)
	}

	logger.Info("Replica %d sending view-change, v:%d, h:%d, |C|:%d, |P|:%d, |Q|:%d",
		instance.id, vc.View, vc.H, len(vc.Cset), len(vc.Pset), len(vc.Qset))

	return instance.broadcast(&Message{&Message_ViewChange{vc}}, false)
}

func (instance *Plugin) recvViewChange(vc *ViewChange) error {
	logger.Info("Replica %d received view-change, v:%d, h:%d, |C|:%d, |P|:%d, |Q|:%d",
		instance.id, vc.View, vc.H, len(vc.Cset), len(vc.Pset), len(vc.Qset))

	return nil
}
