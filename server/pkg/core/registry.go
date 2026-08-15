package core

import (
	"errors"
	"fmt"
)

var ErrModuleRegistered = errors.New("module already registered")

// Registry manages module registration
type Registry struct {
	modules map[string]Module
}

func NewRegistry() *Registry {
	return &Registry{modules: make(map[string]Module)}
}

// Register asserts the module registratio, causing panic on duplicates
func (r *Registry) Register(m Module) {
	if _, ok := r.modules[m.Id()]; ok {
		panic(fmt.Errorf("%s: %w", m.Id(), ErrModuleRegistered))
	}

	r.modules[m.Id()] = m
}

func (r *Registry) All() []Module {
	ms := make([]Module, len(r.modules))
	i := 0
	for _, v := range r.modules {
		ms[i] = v
		i++
	}
	return ms
}
