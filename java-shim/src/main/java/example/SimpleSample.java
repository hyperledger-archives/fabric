package example;

import shim.ChaincodeBase;
import shim.ChaincodeStub;

/**
 * <h1>Classic "transfer" sample chaincode</h1>
 * (java implementation of <A href="https://github.com/hyperledger/fabric/blob/master/examples/chaincode/go/chaincode_example02/chaincode_example02.go">chaincode_example02.go</A>)
 * @author Sergey Pomytkin spomytkin@gmail.com
 *
 */
public class SimpleSample extends ChaincodeBase {

	@Override
	public String run(ChaincodeStub stub, String function, String[] args) {
		switch (function) {
		case "init":
			init(stub, function, args);
			break;
		case "transfer":
			transfer(stub, args);
			break;			
		case "put":
			for (int i = 0; i < args.length; i += 2)
				stub.putState(args[i], args[i + 1]);
			break;
		case "del":
			for (String arg : args)
				stub.delState(arg);
			break;
		default: 
			return transfer(stub, args);
		}
	 
		return null;
	}

	private String  transfer(ChaincodeStub stub, String[] args) {
		if(args.length!=3){
			return "{\"Error\":\"Incorrect number of arguments. Expecting 3: from, to, amount\"}";
		}
		String fromName =args[0];
		String fromAm=stub.getState(fromName);
		String toName =args[1];
		String toAm=stub.getState(toName);
		String am =stub.getState(args[2]);
		int valFrom=0;
		if (fromAm!=null&&!fromAm.isEmpty()){			
			try{
				valFrom = Integer.parseInt(fromAm);
			}catch(NumberFormatException e ){
				return "{\"Error\":\"Expecting integer value for asset holding of "+fromName+" \"}";		
			}		
		}else{
			return "{\"Error\":\"Failed to get state for " +fromName + "\"}";
		}

		int valTo=0;
		if (toAm!=null&&!toAm.isEmpty()){			
			try{
				valTo = Integer.parseInt(toAm);
			}catch(NumberFormatException e ){
				return "{\"Error\":\"Expecting integer value for asset holding of "+toName+" \"}";		
			}		
		}else{
			return "{\"Error\":\"Failed to get state for " +toName + "\"}";
		}
		
		int valA =0;
		try{
			valA = Integer.parseInt(am);
		}catch(NumberFormatException e ){
			return "{\"Error\":\"Expecting integer value for ammount \"}";		
		}		
		if(valA>valFrom)
			return "{\"Error\":\"Insuficient asset holding value for requested transfer ammount \"}";		
		valFrom = valFrom-valA;
		valTo = valTo+valA;
		stub.putState(fromName,""+ valFrom);
		stub.putState(toName, ""+valTo);		
		return null;
		
	}

	public String init(ChaincodeStub stub, String function, String[] args) {
		if(args.length!=4){
			return "{\"Error\":\"Incorrect number of arguments. Expecting 4\"}";
		}
		try{
			int valA = Integer.parseInt(args[1]);
			int valB = Integer.parseInt(args[3]);
			stub.putState(args[0], args[1]);
			stub.putState(args[2], args[3]);		
		}catch(NumberFormatException e ){
			return "{\"Error\":\"Expecting integer value for asset holding\"}";
		}		
		return null;
	}

	
	@Override
	public String query(ChaincodeStub stub, String function, String[] args) {
		if(args.length!=1){
			return "{\"Error\":\"Incorrect number of arguments. Expecting name of the person to query\"}";
		}
		String am =stub.getState(args[0]);
		if (am!=null&&!am.isEmpty()){
			try{
				int valA = Integer.parseInt(am);
				return  "{\"Name\":\"" + args[0] + "\",\"Amount\":\"" + am + "\"}";
			}catch(NumberFormatException e ){
				return "{\"Error\":\"Expecting integer value for asset holding\"}";		
			}		}else{
			return "{\"Error\":\"Failed to get state for " + args[0] + "\"}";
		}
		

	}

	@Override
	public String getChaincodeID() {
		return "SimpleSample";
	}

	public static void main(String[] args) throws Exception {
		new SimpleSample().start(args);
	}


}
