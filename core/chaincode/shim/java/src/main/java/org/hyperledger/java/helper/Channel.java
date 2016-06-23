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
