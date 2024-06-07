package business

import (
	"context"

	"github.com/xiaoshicae/go-easy-extension/core/errors"
)

type DefaultBusinessManager struct {
	businessMap map[string]IBusiness
}

func (d *DefaultBusinessManager) RegisterAbility(ctx context.Context, business IBusiness) error {
	if business == nil {
		return errors.BusinessNilErr
	}
	if d.businessMap == nil {
		d.businessMap = make(map[string]IBusiness)
	}
	if _, ok := d.businessMap[business.Code()]; ok {
		return errors.BusinessRegisterDuplicateErr
	}
	d.businessMap[business.Code()] = business
	return nil
}

func (d *DefaultBusinessManager) GetBusiness(ctx context.Context, code string) (IBusiness, error) {
	if d.businessMap == nil {
		return nil, errors.BusinessNotFoundErr
	}
	business, ok := d.businessMap[code]
	if !ok {
		return nil, errors.BusinessNotFoundErr
	}
	return business, nil
}

func (d *DefaultBusinessManager) ListAllIBusinesses(ctx context.Context) []IBusiness {
	if d.businessMap == nil {
		return nil
	}
	businesses := make([]IBusiness, 0)
	for _, business := range d.businessMap {
		businesses = append(businesses, business)
	}
	return businesses
}
