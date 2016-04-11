package fsm.exceptions;

public class CancelledException extends Exception {

	public final Exception error;
	
	public CancelledException() {
		this(null);
	}
	
	public CancelledException(Exception error) {
		super("The transition was cancelled" + error == null ?
				"" : " with error " + error.toString());
		this.error = error;
	}
	
}
