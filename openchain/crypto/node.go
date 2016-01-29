package crypto

// Utility functions

func eTypeToString(eType Entity_Type) string {
	switch eType {
	case Entity_Client:
		return "client"
	case Entity_Peer:
		return "peer"
	case Entity_Validator:
		return "validator"
	}
	return "Invalid Type"
}
