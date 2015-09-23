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

import "github.com/looplab/fsm"

type PeerConnectionFSM struct {
	To  string
	FSM *fsm.FSM
}

func NewPeerConnectionFSM(to string) *PeerConnectionFSM {
	d := &PeerConnectionFSM{
		To: to,
	}

	d.FSM = fsm.NewFSM(
		"created",
		fsm.Events{
			{Name: "HELLO", Src: []string{"created"}, Dst: "established"},
			{Name: "GET_PEERS", Src: []string{"established"}, Dst: "established"},
			{Name: "PEERS", Src: []string{"established"}, Dst: "established"},
			{Name: "PING", Src: []string{"established"}, Dst: "established"},
			{Name: "DISCONNECT", Src: []string{"created", "established"}, Dst: "closed"},
		},
		fsm.Callbacks{
			"enter_state": func(e *fsm.Event) { d.enterState(e) },
		},
	)

	return d
}

func (d *PeerConnectionFSM) enterState(e *fsm.Event) {
	log.Debug("The bi-directional stream to %s is %s\n", d.To, e.Dst)
}
