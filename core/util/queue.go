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

package util

import "sync"

// Queue is a classic implmentation of a FIFO queue object
type Queue struct {
	head  *element
	tail  *element
	count int
	lock  *sync.Mutex
}

// Element is a single linked list with a data field containing any data
// structure from the caller
type element struct {
	data interface{}
	next *element
}

// NewQueue is a constructor returning a Queue object
func NewQueue() *Queue {
	q := &Queue{}
	q.lock = &sync.Mutex{}
	return q
}

// Size returns the number of elements on the queue
func (q *Queue) Size() int {
	q.lock.Lock()
	defer q.lock.Unlock()
	return q.count
}

// Push appends an item on the queue
// @param item - any application data
func (q *Queue) Push(item interface{}) {
	q.lock.Lock()
	defer q.lock.Unlock()

	element := &element{data: item}

	// Move tail along where t.next points the new element
	if q.tail == nil {
		q.tail = element
		q.head = element
	} else {
		q.tail.next = element
		q.tail = element
	}
	q.count++
}

// Pop returns the data item at the head of the queue
// Example e := q.Pop().(T)  -- cast e to type T
func (q *Queue) Pop() interface{} {
	q.lock.Lock()
	defer q.lock.Unlock()

	if q.head == nil {
		return nil
	}

	// Move head to the next elment
	element := q.head
	q.head = element.next

	// If nothing left on the queue, nilify for garbage collection
	if q.head == nil {
		q.tail = nil
	}
	q.count--

	return element.data
}

// Peek returns the data at the head of the queue
func (q *Queue) Peek() interface{} {
	q.lock.Lock()
	defer q.lock.Unlock()

	element := q.head
	if element == nil {
		return nil
	}

	return element.data
}
