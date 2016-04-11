package helper;

import java.io.PrintStream;

public class SimpleLogger {

	public static final int DEBUG = 0;
	public static final int INFO = 1;
	public static final int ERROR = 2;
	
	private static final String[] NAME = {"DEBUG", "INFO", "ERROR"};
	
	private static SimpleLogger instance;
	
	private int level = DEBUG;
	
	public static SimpleLogger get() {
		if (instance == null)
			instance = new SimpleLogger();
		return instance;
	}
	
	private SimpleLogger() {}
	
	public void setLevel(int level) {
		this.level = level;
	}
	
	public void debug(String debug) { log(DEBUG, debug); }
	public void debug(String debug, Object... args) { log(DEBUG, debug, args); }
	
	public void info(String info) { log(INFO, info); }
	public void info(String info, Object... args) { log(INFO, info, args); }
	
	public void error(String error) { log(ERROR, error); }
	public void error(String error, Object... args) { log(ERROR, error, args); }

	public void log(int level, String message) { log(level, "%s", message); }
	
	public synchronized void log(int level, String format, Object... args) {
		if (level < this.level) return;
		
		StackTraceElement caller = null;
		StackTraceElement[] trace = Thread.currentThread().getStackTrace();
        for (StackTraceElement e : trace) {
            if (SimpleLogger.class.getName().equals(e.getClassName())) continue;
            else if (Thread.class.getName().equals(e.getClassName())) continue;
            else {
            	caller = e;
            	break;
            }
        }
		
        String tmp = String.format(NAME[level] + ": " + format + "\n", args);

        PrintStream out = System.out;
		if (level >= ERROR) {
			out = System.err;
			tmp = caller.toString() + "|" + tmp;
		}
		
		while (tmp.endsWith("\n")) tmp = tmp.substring(0, tmp.length() - 1);
		out.println(tmp);
	}
}
