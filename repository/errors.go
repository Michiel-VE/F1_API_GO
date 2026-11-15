package repository

import "errors"

// ErrDuplicate is returned when an insert violates a unique constraint (needs
// TranslateError: true in the gorm config, see main.go). Services map it to a
// friendly "already exists" error instead of a 500.
var ErrDuplicate = errors.New("duplicate record")
