package org.hyperledger.java.fsm.exceptions;

public class AsyncException extends Exception {

	public final Exception error;
	
	public AsyncException() {
		this(null);
	}
	
	public AsyncException(Exception error) {
		super("Async started" + error == null ?
				"" : " with error " + error.toString());
		this.error = error;
	}
	
}
