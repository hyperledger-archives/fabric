package fsm.exceptions;

public class InTrasistionException extends Exception {

	public final String event;
	
	public InTrasistionException(String event) {
		super("Event '" + event + "' is inappropriate because"
				+ " the previous trasaction had not completed");
		this.event = event;
	}
	
}
