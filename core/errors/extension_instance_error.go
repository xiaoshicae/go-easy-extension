package errors

import "errors"

var ExtensionInstanceNilErr = errors.New("extension instance is nil")
var ExtensionInstanceInvalidErr = errors.New("extension instance invalid, not implements given interfaces")
var ExtensionInstanceRegisterDuplicateErr = errors.New("extension instance already exists")
var ExtensionInstanceNotFoundErr = errors.New("extension instance not found")
var ExtensionFirstMatchedInstanceNotFoundErr = errors.New("extension first match instance  not found")
var ExtensionFirstMatchedInstanceMultilFoundErr = errors.New("extension first match instance multi found")
