package disseminator

// is string 'a' a member of string list 'list'?
func member(a string, list []string) bool {
	for i := range list {
		if a == list[i] {
			return true
		}
	}
	return false
}

// do lists 'a' and 'b' contain a member in common?
func incommon(a, b []string) bool {
	for i := range a {
		if member(a[i], b) {
			return true
		}
	}
	return false
}
