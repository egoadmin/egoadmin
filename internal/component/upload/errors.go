package upload

import "errors"

var (
	ErrObjectNotFound     = errors.New("upload: object not found")
	ErrReferenceNotFound  = errors.New("upload: reference not found")
	ErrReferenceForbidden = errors.New("upload: reference forbidden")
)
