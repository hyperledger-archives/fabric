package fsm.exceptions;

public class InvalidEventException extends Exception {

	public final String event;
	public final String state;
	
	public InvalidEventException(String event, String state) {
		super("Event '" + event + "' is innappropriate"
				+ " given the current state, " + state);
		this.event = event;
		this.state = state;
	}
	
}
