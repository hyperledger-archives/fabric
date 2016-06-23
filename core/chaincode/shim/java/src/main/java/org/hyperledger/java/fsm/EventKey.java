package org.hyperledger.java.fsm;

/** Key for the transition map */
public class EventKey {

	/** The name of the event that the key refers to */
	public final String event;
	
	/** The source from where the event can transition */
	public final String src;
	
	public EventKey(String event, String src) {
		this.event = event;
		this.src = src;
	}

	@Override
	public int hashCode() {
		final int prime = 31;
		int result = 1;
		result = prime * result + ((event == null) ? 0 : event.hashCode());
		result = prime * result + ((src == null) ? 0 : src.hashCode());
		return result;
	}
	
	@Override
	public boolean equals(Object obj) {
		if (this == obj)
			return true;
		if (obj == null)
			return false;
		if (getClass() != obj.getClass())
			return false;
		EventKey other = (EventKey) obj;
		if (event == null) {
			if (other.event != null)
				return false;
		} else if (!event.equals(other.event))
			return false;
		if (src == null) {
			if (other.src != null)
				return false;
		} else if (!src.equals(other.src))
			return false;
		return true;
	}
	
}
