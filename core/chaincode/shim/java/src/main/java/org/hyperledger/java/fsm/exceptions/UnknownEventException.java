package org.hyperledger.java.fsm.exceptions;

public class UnknownEventException extends Exception {

	public final String event;
	
	public UnknownEventException(String event) {
		super("Event '" + event + "' does not exist");
		this.event = event;
	}
	
}
