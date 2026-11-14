package repository

import "strings"

// escapeLike makes user input safe to embed in a LIKE pattern. Without it,
// a request for the name "%" matches every row, and "_" matches any character.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
