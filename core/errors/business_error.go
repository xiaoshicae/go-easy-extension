package errors

import "errors"

var BusinessNilErr = errors.New("business is nil")
var BusinessRegisterDuplicateErr = errors.New("business already exists")
var BusinessNotFoundErr = errors.New("business not found")
