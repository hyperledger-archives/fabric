package org.hyperledger.java.fsm.exceptions;

public class NotInTransitionException extends Exception {

	public NotInTransitionException() {
		super("The transition is inappropriate"
				+ " because there is no state change in progress");
	}
	
}
