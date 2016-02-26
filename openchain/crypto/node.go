package crypto

// Utility functions

func eTypeToString(eType NodeType) string {
	switch eType {
	case NodeClient:
		return "client"
	case NodePeer:
		return "peer"
	case NodeValidator:
		return "validator"
	}
	return "Invalid Type"
}
