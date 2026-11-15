package security

// IsSafeReturnPath only accepts a path on our own site (used for the
// "return to where you were" redirect after login). It rejects "//host" and
// "/\host" (browsers treat a backslash like a slash), control characters and
// anything that is not an absolute path.
func IsSafeReturnPath(p string) bool {
	if p == "" || len(p) > 512 || p[0] != '/' {
		return false
	}
	if len(p) > 1 && (p[1] == '/' || p[1] == '\\') {
		return false
	}
	for _, r := range p {
		if r < 0x20 || r == 0x7f || r == '\\' {
			return false
		}
	}
	return true
}
