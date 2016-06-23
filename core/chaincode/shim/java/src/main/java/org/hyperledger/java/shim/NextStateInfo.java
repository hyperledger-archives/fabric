package org.hyperledger.java.shim;

import protos.Chaincode.ChaincodeMessage;

public class NextStateInfo {

	public ChaincodeMessage message;
	public boolean sendToCC;
	
	public NextStateInfo(ChaincodeMessage message, boolean sendToCC) {
		this.message = message;
		this.sendToCC = sendToCC;
	}
	
}
