package disseminator

import (
	"strings"
)

/* the remaining functions were used for Id checks. they may not be needed */
func isCurateId(s string) bool { return scanId(s, "eeddeeddede") }
func isVecnetId(s string) bool { return scanId(s, "eeddeedde") }

const (
	noidx string = "0123456789bcdfghjkmnpqrstvwxz"
)

// Compare an id against template character by character.
// An 'd' in template must match with a digit in id.
// An 'e' in template must match with a noid alphanumeric character in id,
// which consist of "0123456789bcdfghjkmnpqrstvwxz"
// returns false if any match fails, Otherwise returns true
func scanId(id, template string) bool {
	if len(id) != len(template) {
		return false
	}

	for i := range id {
		var allowed string = "0123456789"
		if template[i] == 'e' {
			allowed = noidx
		}
		if strings.IndexByte(allowed, id[i]) < 0 {
			return false
		}
	}
	return true
}
