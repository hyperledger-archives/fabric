package example;

import shim.ChaincodeBase;
import shim.ChaincodeStub;

public class Example extends ChaincodeBase {

	@Override
	public String run(ChaincodeStub stub, String function, String[] args) {
		switch (function) {
		case "put":
			for (int i = 0; i < args.length; i += 2)
				stub.putState(args[i], args[i + 1]);
			break;
		case "del":
			for (String arg : args)
				stub.delState(arg);
			break;
		case "hello":
			System.out.println("hello invoked");
			break;
		}
		return null;
	}

	@Override
	public String query(ChaincodeStub stub, String function, String[] args) {
		System.out.println("Hello world! query:"+args[0]);
		if (stub.getState(args[0])!=null&&!stub.getState(args[0]).isEmpty()){
			return "Hello world! from "+ stub.getState(args[0]);
		}else{
			return "Hello "+args[0]+"!";
		}
	}

	@Override
	public String getChaincodeID() {
		return "hello";
	}

	public static void main(String[] args) throws Exception {
		System.out.println("Hello world! starting "+args);

		new Example().start(args);
	}


}
