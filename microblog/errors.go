package microblog

import "errors"

var (
	ErrStorage  = errors.New("storage_error")
	ErrNotFound = errors.New("not_found")
)
