package core

import (
	"errors"
	"fmt"
	"reflect"
)

var (
	ErrInitializingModule = errors.New("failed to initialize")
	ErrValidatingModule   = errors.New("failed to validate")
)

// GetModule finds the module in the context asserting the type
func GetModule[T any](ctx *Context, m any) (T, error) {
	var placeholder T

	mod, err := ctx.Module(m)
	if err != nil {
		return placeholder, err
	}

	if actualMod, ok := mod.(T); ok {
		return actualMod, nil
	}

	return placeholder, errors.New("invalid module type")
}

// Context is received as a param to module Provision
type Context struct {
	registry  *Registry
	instances map[string]any
	Config    Config
}

func NewContext(r *Registry, config Config) *Context {
	return &Context{
		registry:  r,
		instances: make(map[string]any),
		Config:    config,
	}
}

func (c *Context) Module(m any) (any, error) {
	mods, err := c.Modules(m)
	if err != nil {
		return nil, err
	}

	if len(mods) == 0 {
		return nil, errors.New("module not found")
	}

	if len(mods) > 1 {
		return nil, errors.New("only one module expected")
	}

	return mods[0], nil
}

func (c *Context) Modules(m any) ([]any, error) {
	k := reflect.TypeOf(m).Elem()

	var mods []any

	modules := c.registry.All()
	for _, mod := range modules {
		instance := mod.New()
		ito := reflect.TypeOf(instance)

		// Verify if concrete type matches desired one
		ok := ito == reflect.TypeOf(m)

		// Otherwise verify if it implements the desired interface
		if !ok && k.Kind() == reflect.Interface {
			ok = ito.Implements(k)
		}

		if ok {
			// Verify if the module was already initialized
			inst, ready := c.instances[mod.Id()]
			if !ready {
				if err := c.load(mod.Id(), instance); err != nil {
					return nil, err
				}
				inst = instance
			}
			mods = append(mods, inst)
		}
	}

	return mods, nil
}

func (c *Context) load(id string, instance any) error {
	if p, ok := instance.(Initializer); ok {
		if err := p.Initialize(c); err != nil {
			return fmt.Errorf("%w %s: %w", ErrInitializingModule, id, err)
		}
	}

	if v, ok := instance.(Validator); ok {
		if err := v.Validate(); err != nil {
			return fmt.Errorf("%w %s: %w", ErrValidatingModule, id, err)
		}
	}

	c.instances[id] = instance

	return nil
}
