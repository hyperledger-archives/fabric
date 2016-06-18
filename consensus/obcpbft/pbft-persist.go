/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package obcpbft

import (
	"encoding/base64"
	"fmt"

	"github.com/golang/protobuf/proto"
)

func (instance *pbftCore) persistQSet() {
	var qset []*ViewChange_PQ

	for _, q := range instance.calcQSet() {
		qset = append(qset, q)
	}

	instance.persistPQSet("qset", qset)
}

func (instance *pbftCore) persistPSet() {
	var pset []*ViewChange_PQ

	for _, p := range instance.calcPSet() {
		pset = append(pset, p)
	}

	instance.persistPQSet("pset", pset)
}

func (instance *pbftCore) persistPQSet(key string, set []*ViewChange_PQ) {
	raw, err := proto.Marshal(&PQset{set})
	if err != nil {
		logger.Warningf("Replica %d could not persist pqset: %s", instance.id, err)
		return
	}
	instance.consumer.StoreState(key, raw)
}

func (instance *pbftCore) restorePQSet(key string) []*ViewChange_PQ {
	raw, err := instance.consumer.ReadState(key)
	if err != nil {
		logger.Debugf("Replica %d could not restore state %s: %s", instance.id, key, err)
		return nil
	}
	val := &PQset{}
	err = proto.Unmarshal(raw, val)
	if err != nil {
		logger.Errorf("Replica %d could not unmarshal %s - local state is damaged: %s", instance.id, key, err)
		return nil
	}
	return val.GetSet()
}

func (instance *pbftCore) persistRequest(digest string) {
	req := instance.reqStore[digest]
	raw, err := proto.Marshal(req)
	if err != nil {
		logger.Warningf("Replica %d could not persist request: %s", instance.id, err)
		return
	}
	instance.consumer.StoreState("req."+digest, raw)
}

func (instance *pbftCore) persistDelRequest(digest string) {
	instance.consumer.DelState("req." + digest)
}

func (instance *pbftCore) persistDelAllRequests() {
	reqs, err := instance.consumer.ReadStateSet("req.")
	if err == nil {
		for k := range reqs {
			instance.consumer.DelState(k)
		}
	}
}

func (instance *pbftCore) persistCheckpoint(seqNo uint64, id []byte) {
	key := fmt.Sprintf("chkpt.%d", seqNo)
	instance.consumer.StoreState(key, id)
}

func (instance *pbftCore) persistDelCheckpoint(seqNo uint64) {
	key := fmt.Sprintf("chkpt.%d", seqNo)
	instance.consumer.DelState(key)
}

func (instance *pbftCore) restoreState() {
	updateSeqView := func(set []*ViewChange_PQ) {
		for _, e := range set {
			if instance.view < e.View {
				instance.view = e.View
			}
			if instance.seqNo < e.SequenceNumber {
				instance.seqNo = e.SequenceNumber
			}
		}
	}

	set := instance.restorePQSet("pset")
	for _, e := range set {
		instance.pset[e.SequenceNumber] = e
	}
	updateSeqView(set)

	set = instance.restorePQSet("qset")
	for _, e := range set {
		instance.qset[qidx{e.Digest, e.SequenceNumber}] = e
	}
	updateSeqView(set)

	reqs, err := instance.consumer.ReadStateSet("req.")
	if err == nil {
		for k, v := range reqs {
			req := &Request{}
			err = proto.Unmarshal(v, req)
			if err != nil {
				logger.Warningf("Replica %d could not restore request %s", instance.id, k)
			} else {
				instance.reqStore[hashReq(req)] = req
			}
		}
	} else {
		logger.Warningf("Replica %d could not restore reqStore: %s", instance.id, err)
	}

	chkpts, err := instance.consumer.ReadStateSet("chkpt.")
	if err == nil {
		highSeq := uint64(0)
		for key, id := range chkpts {
			var seqNo uint64
			if _, err = fmt.Sscanf(key, "chkpt.%d", &seqNo); err != nil {
				logger.Warningf("Replica %d could not restore checkpoint key %s", instance.id, key)
			} else {
				idAsString := base64.StdEncoding.EncodeToString(id)
				logger.Debugf("Replica %d found checkpoint %s for seqNo %d", instance.id, idAsString, seqNo)
				instance.chkpts[seqNo] = idAsString
				if seqNo > highSeq {
					highSeq = seqNo
				}
			}
		}
		instance.moveWatermarks(highSeq)
	} else {
		logger.Warningf("Replica %d could not restore checkpoints: %s", instance.id, err)
	}

	instance.restoreLastSeqNo()

	logger.Infof("Replica %d restored state: view: %d, seqNo: %d, pset: %d, qset: %d, reqs: %d, chkpts: %d",
		instance.id, instance.view, instance.seqNo, len(instance.pset), len(instance.qset), len(instance.reqStore), len(instance.chkpts))
}

func (instance *pbftCore) restoreLastSeqNo() {
	var err error
	if instance.lastExec, err = instance.consumer.getLastSeqNo(); err != nil {
		logger.Warningf("Replica %d could not restore lastExec: %s", instance.id, err)
		instance.lastExec = 0
	}
	logger.Infof("Replica %d restored lastExec: %d", instance.id, instance.lastExec)
}
