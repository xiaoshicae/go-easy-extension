package errors

import "errors"

var AbilityNilErr = errors.New("ability is nil")
var AbilityRegisterDuplicateErr = errors.New("ability already exists")
var AbilityRegisterNotDefaultAbilityErr = errors.New("should register default ability")
var AbilityNotFoundErr = errors.New("ability not found")
