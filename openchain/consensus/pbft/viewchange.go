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

func (instance *Plugin) correctViewChange(vc *ViewChange) bool {
	for _, p := range append(vc.Pset, vc.Qset...) {
		if !(p.View < vc.View && p.SequenceNumber > vc.H && p.SequenceNumber <= vc.H+instance.L) {
			logger.Debug("invalid p entry in view-change: vc(v:%d h:%d) p(v:%d n:%d)",
				vc.View, vc.H, p.View, p.SequenceNumber)
			return false
		}
	}

	for _, c := range vc.Cset {
		// XXX the paper says c.n > vc.h
		if !(c.SequenceNumber >= vc.H && c.SequenceNumber <= vc.H+instance.L) {
			logger.Debug("invalid c entry in view-change: vc(v:%d h:%d) c(n:%d)",
				vc.View, vc.H, c.SequenceNumber)
			return false
		}
	}

	return true
}

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

	// clear old messages
	for idx, _ := range instance.certStore {
		if idx.v < instance.view {
			delete(instance.certStore, idx)
			// XXX how do we clear reqStore?
		}
	}
	for idx, _ := range instance.viewChangeStore {
		if idx.v < instance.view {
			delete(instance.viewChangeStore, idx)
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

	return instance.broadcast(&Message{&Message_ViewChange{vc}}, true)
}

func (instance *Plugin) recvViewChange(vc *ViewChange) error {
	logger.Info("Replica %d received view-change from replica %d, v:%d, h:%d, |C|:%d, |P|:%d, |Q|:%d",
		instance.id, vc.ReplicaId, vc.View, vc.H, len(vc.Cset), len(vc.Pset), len(vc.Qset))

	if !(vc.View >= instance.view && instance.correctViewChange(vc) || instance.viewChangeStore[vcidx{vc.View, vc.ReplicaId}] != nil) {
		logger.Warning("View-change message incorrect")
		return nil
	}

	instance.viewChangeStore[vcidx{vc.View, vc.ReplicaId}] = vc

	return nil
}

func (instance *Plugin) selectInitialCheckpoint() (checkpoint uint64, ok bool) {
	checkpoints := make(map[ViewChange_C][]*ViewChange)
	for _, vc := range instance.viewChangeStore {
		for _, c := range vc.Cset {
			checkpoints[*c] = append(checkpoints[*c], vc)
		}
	}

	if len(checkpoints) == 0 {
		logger.Debug("no checkpoints to select from: %d %s",
			len(instance.viewChangeStore), checkpoints)
		return
	}

	for idx, vcList := range checkpoints {
		// need weak certificate for the checkpoint
		if len(vcList) <= instance.f {
			logger.Debug("no weak certificate for n:%d",
				idx.SequenceNumber)
			continue
		}

		quorum := 0
		for _, vc := range vcList {
			if vc.H <= idx.SequenceNumber {
				quorum += 1
			}
		}

		if quorum <= 2*instance.f {
			logger.Debug("no quorum for n:%d",
				idx.SequenceNumber)
			continue
		}

		if checkpoint <= idx.SequenceNumber {
			checkpoint = idx.SequenceNumber
			ok = true
		}
	}

	return
}

func (instance *Plugin) assignSequenceNumbers(h uint64) (msgList map[uint64]string) {
	msgList = make(map[uint64]string)

	// "for all n such that h < n <= h + L"
nLoop:
	for n := h + 1; n < h+instance.L; n++ {
		// "∃m ∈ S..."
		for _, m := range instance.viewChangeStore {
			// "...with <n,d,v> ∈ m.P"
			for _, em := range m.Pset {
				quorum := 0
				// "A1. ∃2f+1 messages m' ∈ S"
				for _, mp := range instance.viewChangeStore {
					if mp.H >= n {
						continue
					}
					// "∀<n,d',v'> ∈ m'.P"
					for _, emp := range mp.Pset {
						if n == emp.SequenceNumber && emp.View < em.View || (emp.View == em.View && emp.Digest == em.Digest) {
							quorum += 1
						}
					}
				}

				if quorum < 2*instance.f+1 {
					continue
				}

				quorum = 0
				// "A2. ∃f+1 messages m' ∈ S"
				for _, mp := range instance.viewChangeStore {
					// "∃<n,d',v'> ∈ m'.Q"
					for _, emp := range mp.Qset {
						if n == emp.SequenceNumber && emp.View >= em.View && emp.Digest == em.Digest {
							quorum += 1
						}
					}
				}

				if quorum < instance.f+1 {
					continue
				}

				// "then select the null request with digest d for number n"
				msgList[n] = em.Digest

				continue nLoop
			}
		}

		quorum := 0
		// "else if ∃2f+1 messages m ∈ S"
	nullLoop:
		for _, m := range instance.viewChangeStore {
			// "m.P has no entry"
			for _, em := range m.Pset {
				if em.SequenceNumber == n {
					continue nullLoop
				}
			}
			quorum += 1
		}

		if quorum >= 2*instance.f+1 {
			// "then select the null request for number n"
			msgList[n] = ""
		}
	}

	return
}
