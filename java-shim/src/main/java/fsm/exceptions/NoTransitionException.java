package fsm.exceptions;

public class NoTransitionException extends Exception {

	public final Exception error;
	
	public NoTransitionException() {
		this(null);
	}
	
	public NoTransitionException(Exception error) {
		super("No transition occurred" + (error == null ? "" : " because of error " + error.toString()));
		this.error = error;
	}
	
}
