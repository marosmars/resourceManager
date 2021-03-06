package pools

import (
	"context"
	"github.com/marosmars/resourceManager/ent"
	resource "github.com/marosmars/resourceManager/ent/resource"
	resourcePool "github.com/marosmars/resourceManager/ent/resourcepool"
	"github.com/pkg/errors"
)

// NewSetPool creates a brand new pool allocating DB entities in the process
func NewSetPool(
	ctx context.Context,
	client *ent.Client,
	resourceType *ent.ResourceType,
	propertyValues []RawResourceProps,
	poolName string) (Pool, error) {
	pool, _, err := NewSetPoolFull(ctx, client, resourceType, propertyValues, poolName)
	return pool, err
}

//creates a brand new pool + returns the pools underlying meta information
func NewSetPoolFull(
	ctx context.Context,
	client *ent.Client,
	resourceType *ent.ResourceType,
	propertyValues []RawResourceProps,
	poolName string) (Pool, *ent.ResourcePool, error) {

	// TODO check that propertyValues are unique

	pool, err := newPoolInner(ctx, client, resourceType, propertyValues, poolName, resourcePool.PoolTypeSet)

	if err != nil {
		return nil, nil, err
	}

	return &SetPool{pool, ctx, client}, pool, nil
}

// Destroy removes the pool from DB if there are no more claims
func (pool SetPool) Destroy() error {
	// Check if there are no more claims
	claims, err := pool.QueryResources()
	if err != nil {
		return err
	}

	if len(claims) > 0 {
		return errors.Errorf("Unable to destroy pool \"%s\", there are claimed resources",
			pool.Name)
	}

	// Delete props
	resources, err := pool.FindResources().All(pool.ctx)
	if err != nil {
		return errors.Wrapf(err, "Cannot destroy pool \"%s\". Unable to cleanup resoruces", pool.Name)
	}
	for _, res := range resources {
		props, err := res.QueryProperties().All(pool.ctx)
		if err != nil {
			return errors.Wrapf(err, "Cannot destroy pool \"%s\". Unable to cleanup resoruces", pool.Name)
		}

		for _, prop := range props {
			pool.client.Property.DeleteOne(prop).Exec(pool.ctx)
		}
		if err != nil {
			return errors.Wrapf(err, "Cannot destroy pool \"%s\". Unable to cleanup resoruces", pool.Name)
		}
	}

	// Delete resources
	_, err = pool.client.Resource.Delete().Where(resource.HasPoolWith(resourcePool.ID(pool.ID))).Exec(pool.ctx)
	if err != nil {
		return errors.Wrapf(err, "Cannot destroy pool \"%s\". Unable to cleanup resoruces", pool.Name)
	}

	// Delete pool itself
	err = pool.client.ResourcePool.DeleteOne(pool.ResourcePool).Exec(pool.ctx)
	if err != nil {
		return errors.Wrapf(err, "Cannot destroy pool \"%s\"", pool.Name)
	}

	return nil
}

func (pool SetPool) AddLabel(label PoolLabel) error {
	// TODO implement labeling
	return errors.Errorf("NOT IMPLEMENTED")
}

func (pool SetPool) ClaimResource() (*ent.Resource, error) {
	// Allocate new resource for this tag
	unclaimedRes, err := pool.queryUnclaimedResourceEager()
	if err != nil {
		return nil, errors.Wrapf(err, "Unable to find unclaimed resource in pool \"%s\"",
			pool.Name)
	}

	err = pool.client.Resource.UpdateOne(unclaimedRes).SetClaimed(true).Exec(pool.ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "Unable to claim a resource in pool \"%s\"", pool.Name)
	}
	return unclaimedRes, err
}


func (pool SetPool) FreeResource(raw RawResourceProps) error {
	return pool.freeResourceInner(raw)
}

func (pool SetPool) freeResourceInner(raw RawResourceProps) error {
	query, err := pool.FindResource(raw)
	if err != nil {
		return errors.Wrapf(err, "Unable to find resource in pool: \"%s\"", pool.Name)
	}
	res, err := query.
		WithProperties().
		Only(pool.ctx)

	if err != nil {
		return errors.Wrapf(err, "Unable to free a resource in pool \"%s\". Unable to find resource", pool.Name)
	}

	if res.Claimed == false {
		return errors.Wrapf(err, "Unable to free a resource in pool \"%s\". It has not been claimed", pool.Name)
	}

	pool.client.Resource.UpdateOne(res).SetClaimed(false).Exec(pool.ctx)
	if err != nil {
		return errors.Wrapf(err, "Unable to free a resource in pool \"%s\". Unable to unclaim", pool.Name)
	}

	return nil
}

func (pool SetPool) FindResource(raw RawResourceProps) (*ent.ResourceQuery, error) {
	propComparator, err := compareProps(pool.ctx, pool.QueryResourceType().OnlyX(pool.ctx), raw)
	if err != nil {
		return nil, errors.Wrapf(err, "Unable to find resource in pool: \"%s\"", pool.Name)
	}

	return pool.FindResources().
		Where(resource.HasPropertiesWith(propComparator...)), nil
}

func (pool SetPool) QueryResource(raw RawResourceProps) (*ent.Resource, error) {
	query, err := pool.FindResource(raw)
	if err != nil {
		return nil, err
	}
	return query.
		Where(resource.Claimed(true)).
		Only(pool.ctx)
}

// load eagerly with some edges, ready to be copied
func (pool SetPool) queryUnclaimedResourceEager() (*ent.Resource, error) {
	// Find first unclaimed
	resource, err := pool.FindResources().
		Where(resource.Claimed(false)).
		First(pool.ctx)

	// No more unclaimed
	if ent.IsNotFound(err) {
		return nil, errors.Wrapf(err, "No more free resources in the pool: \"%s\"", pool.Name)
	}

	return resource, err
}

func (pool SetPool) FindResources() *ent.ResourceQuery {
	return pool.client.Resource.Query().
		Where(resource.HasPoolWith(resourcePool.ID(pool.ID)))
}

func (pool SetPool) QueryResources() (ent.Resources, error) {
	resource, err := pool.FindResources().
		Where(resource.Claimed(true)).
		All(pool.ctx)

	return resource, err
}

