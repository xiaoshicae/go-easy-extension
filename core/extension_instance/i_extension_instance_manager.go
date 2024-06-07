package extension_instance

import "context"

type IExtensionInstanceManager interface {
	RegisterExtension(ctx context.Context, instance interface{}, implementsExtensionType []ExtensionType, name string) error

	GetExtension(ctx context.Context, extensionType ExtensionType, name string) (interface{}, error)
}

type ExtensionType interface{}
