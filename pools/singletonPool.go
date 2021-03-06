package pools

import (
	"context"
	"github.com/marosmars/resourceManager/ent"
	resourcePool "github.com/marosmars/resourceManager/ent/resourcepool"
)

// NewSingletonPool creates a brand new pool allocating DB entities in the process
func NewSingletonPool(
	ctx context.Context,
	client *ent.Client,
	resourceType *ent.ResourceType,
	propertyValues RawResourceProps,
	poolName string) (Pool, error) {
	pool, _, err := NewSingletonPoolFull(ctx, client, resourceType, propertyValues, poolName)
	return pool, err
}

//creates a brand new pool + returns the pools underlying meta information
func NewSingletonPoolFull(
	ctx context.Context,
	client *ent.Client,
	resourceType *ent.ResourceType,
	propertyValues RawResourceProps,
	poolName string) (Pool, *ent.ResourcePool, error) {

	pool, err := newPoolInner(ctx, client, resourceType, []RawResourceProps{propertyValues}, poolName, resourcePool.PoolTypeSingleton)

	if err != nil {
		return nil, nil, err
	}

	return &SingletonPool{SetPool{pool, ctx, client}}, pool, nil
}

func (pool SingletonPool) ClaimResource() (*ent.Resource, error) {
	return pool.queryUnclaimedResourceEager()
}

func (pool SingletonPool) FreeResource(raw RawResourceProps) error {
	return nil
}

func (pool SingletonPool) QueryResource(raw RawResourceProps) (* ent.Resource, error) {
	return pool.QueryResource(raw)
}

func (pool SingletonPool) QueryResources() (ent.Resources, error) {
	return pool.QueryResources()
}


