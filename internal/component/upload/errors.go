package upload

import "errors"

var ErrObjectNotFound = errors.New("upload: object not found")
var ErrReferenceNotFound = errors.New("upload: reference not found")
var ErrReferenceForbidden = errors.New("upload: reference forbidden")
