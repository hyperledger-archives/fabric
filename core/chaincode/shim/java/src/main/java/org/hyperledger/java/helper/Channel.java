/*
Copyright DTCC 2016 All Rights Reserved.

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

package org.hyperledger.java.helper;

import java.io.Closeable;
import java.util.HashSet;
import java.util.concurrent.LinkedBlockingQueue;

public class Channel<E> extends LinkedBlockingQueue<E> implements Closeable {

	private boolean closed = false;
	
	private HashSet<Thread> waiting = new HashSet<>();
	
	//TODO add other methods to secure closing behavior
	
	@Override
	public E take() throws InterruptedException {
		synchronized (waiting) {
			if (closed) throw new InterruptedException("Channel closed");
			waiting.add(Thread.currentThread());
		}
		E e = super.take();
		synchronized (waiting) {
			waiting.remove(Thread.currentThread());
		}
		return e;
	}
	
	
	@Override
	public boolean add(E e) {
		if (closed) {
			throw new IllegalStateException("Channel is closed");
		}
		return super.add(e);
	}
	
	
	@Override
	public void close() {
		synchronized (waiting) {
			closed = true;
			for (Thread t : waiting) {
				t.interrupt();
			}
			waiting.clear();
			clear();
		}
	}

}
