package validator

func isXMLPrefix(prefix []byte) bool {
	return len(prefix) == 3 && prefix[0] == 'x' && prefix[1] == 'm' && prefix[2] == 'l'
}
