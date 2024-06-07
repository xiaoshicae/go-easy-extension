package extension_instance

import (
	"context"
	"fmt"
	"reflect"

	"github.com/xiaoshicae/go-easy-extension/core/errors"
)

type DefaultExtensionInstanceManager struct {
	extensionInstanceMap map[string]interface{}
}

func (d *DefaultExtensionInstanceManager) RegisterExtension(ctx context.Context, instance interface{}, implementsExtensionType []ExtensionType, name string) error {
	if d.extensionInstanceMap == nil {
		d.extensionInstanceMap = make(map[string]interface{})
	}
	if instance == nil {
		return errors.ExtensionInstanceNilErr
	}

	rt := reflect.TypeOf(instance)

	for _, extensionType := range implementsExtensionType {
		etRt := reflect.TypeOf(extensionType)
		if etRt.Kind() == reflect.Ptr {
			etRt = etRt.Elem()
		}
		if !rt.Implements(etRt) {
			return errors.ExtensionInstanceInvalidErr
		}
		if _, ok := d.extensionInstanceMap[d.makeKey(extensionType, name)]; ok {
			return errors.ExtensionInstanceRegisterDuplicateErr
		}
		d.extensionInstanceMap[d.makeKey(extensionType, name)] = instance
	}
	return nil
}

func (d *DefaultExtensionInstanceManager) GetExtension(ctx context.Context, extensionType ExtensionType, name string) (interface{}, error) {
	if d.extensionInstanceMap == nil {
		return nil, errors.ExtensionInstanceNotFoundErr
	}
	extensionInstance, ok := d.extensionInstanceMap[d.makeKey(extensionType, name)]
	if !ok {
		return nil, errors.ExtensionInstanceNotFoundErr
	}
	return extensionInstance, nil
}

func (d *DefaultExtensionInstanceManager) makeKey(extensionType interface{}, name string) string {
	return fmt.Sprintf("%v#%v", reflect.TypeOf(extensionType).String(), name)
}
