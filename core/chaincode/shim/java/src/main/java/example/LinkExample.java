package example;

import shim.ChaincodeBase;
import shim.ChaincodeStub;

public class LinkExample extends ChaincodeBase {

	//Default name for map chaincode in dev mode
	//Can be set to a hash location via init or setMap 
	private String mapChaincode = "map";
	
	@Override
	public String run(ChaincodeStub stub, String function, String[] args) {
		switch (function) {
		case "init":
		case "setMap":
			mapChaincode = args[0];
			break;
		case "put":
			stub.invokeChaincode(mapChaincode, function, args);			
		default:
			break;
		}
		return null;
	}

	@Override
	public String query(ChaincodeStub stub, String function, String[] args) {
		String tmp = stub.queryChaincode("map", function, args);
		if (tmp.isEmpty()) tmp = "NULL";
		else tmp = "\"" + tmp + "\"";
		tmp += " (queried from map chaincode)";
		return tmp;
	}

	public static void main(String[] args) throws Exception {
		new LinkExample().start(args);
		//new Example().start();
	}

	@Override
	public String getChaincodeID() {
		return "link";
	}
	
}
