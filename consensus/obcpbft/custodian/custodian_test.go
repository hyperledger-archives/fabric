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

package custodian

import (
	"testing"
	"time"
)

type info struct {
	id   string
	data string
}

func TestCustody(t *testing.T) {
	notify := make(chan info)
	c := New(100*time.Millisecond,
		func(req string, data interface{}) {
			notify <- info{req, data.(string)}
		})
	defer c.Stop()

	c.Register("foo", "bar")
	if !c.InCustody("foo") {
		t.Error("should have request in custody")
	}
	if c.InCustody("bar") {
		t.Error("should not have request in custody")
	}
	select {
	case req := <-notify:
		if req.id != "foo" || req.data != "bar" {
			t.Error("invalid notify")
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("did not receive notification")
	}
	if c.InCustody("foo") {
		t.Error("should not have request in custody")
	}
}

func TestRemove(t *testing.T) {
	notify := make(chan info)
	c := New(100*time.Millisecond,
		func(id string, data interface{}) {
			notify <- info{id, data.(string)}
		})
	defer c.Stop()

	c.Register("foo", "1")
	time.Sleep(10 * time.Millisecond)
	c.Register("bar", "2")
	time.Sleep(10 * time.Millisecond)
	if !c.Remove("foo") {
		t.Error("foo not found")
	}
	if c.Remove("foo") {
		t.Error("foo found")
	}
	select {
	case req := <-notify:
		if req.id != "bar" || req.data != "2" {
			t.Error("invalid notify:")
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("did not receive notification")
	}
}

func TestRemoveAll(t *testing.T) {
	notify := make(chan info)
	c := New(100*time.Millisecond,
		func(id string, data interface{}) {
			notify <- info{id, data.(string)}
		})
	defer c.Stop()

	c.Register("foo", "1")
	time.Sleep(10 * time.Millisecond)
	c.RemoveAll()
	select {
	case <-notify:
		t.Error("did received cancelled")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestElements(t *testing.T) {
	notify := make(chan info)
	c := New(100*time.Millisecond,
		func(id string, data interface{}) {
			notify <- info{id, data.(string)}
		})
	defer c.Stop()

	c.Register("foo", "1")
	c.Register("baz", "3")
	time.Sleep(50 * time.Millisecond)
	c.Remove("baz")
	c.Register("bar", "2")

	l := c.Elements()
	if len(l) != 2 {
		t.Fatal("expected 2 elements")
	}
	if l[0].ID != "foo" || l[0].Data.(string) != "1" ||
		l[1].ID != "bar" || l[1].Data.(string) != "2" {
		t.Error("invalid elements")
	}
	select {
	case <-notify:
	case <-time.After(200 * time.Millisecond):
		t.Error("expected expiration")
	}

	l = c.RemoveAll()
	if len(l) != 1 {
		t.Fatal("expected 1 elements")
	}
	if l[0].ID != "bar" || l[0].Data.(string) != "2" {
		t.Error("invalid element")
	}
}

func TestReCustody(t *testing.T) {
	notify := make(chan bool)
	var c *Custodian
	c = New(100*time.Millisecond,
		func(req string, data interface{}) {
			c.Register(req, data)
			notify <- true
		})
	defer c.Stop()

	c.Register("foo", "bar")
	select {
	case <-notify:
		reqs := c.Elements()
		if len(reqs) != 1 || reqs[0].ID != "foo" || reqs[0].Data != "bar" {
			t.Error("invalid datasheet")
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("did not receive notification")
	}
}

func TestBackToBack(t *testing.T) {
	notify := make(chan bool)
	var reqs []string

	var c *Custodian
	c = New(100*time.Millisecond,
		func(req string, data interface{}) {
			reqs = append(reqs, req)
			if len(reqs) == 2 {
				notify <- true
			}
		})
	defer c.Stop()

	c.Register("foo", "bar")
	c.Register("foo2", "bar")
	select {
	case <-notify:
		if len(reqs) != 2 || reqs[0] != "foo" || reqs[1] != "foo2" {
			t.Error("invalid datasheet")
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("did not receive notification")
	}
}
