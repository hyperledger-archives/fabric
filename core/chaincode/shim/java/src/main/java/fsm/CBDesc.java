package fsm;

public class CBDesc {

	public final CallbackType type;
	public final String trigger;
	public final Callback callback;
	
	/**
	 * 
	 * @param type
	 * @param trigger
	 * @param callback
	 */
	public CBDesc(CallbackType type, String trigger, Callback callback) {
		this.type = type;
		this.trigger = trigger;
		this.callback = callback;
	}

}
