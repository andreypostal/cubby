package core

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Greeter is the interface the tests resolve modules through
type Greeter interface {
	Greet() string
}

// greeter counts how many times the module system initialized it, which is how
// the tests tell a singleton from a fresh instance
type greeter struct {
	inits *int
}

func (g *greeter) Id() string    { return "greeter" }
func (g *greeter) New() any      { return &greeter{inits: g.inits} }
func (g *greeter) Greet() string { return "hi" }

func (g *greeter) Initialize(*Context) error {
	if g.inits != nil {
		*g.inits++
	}
	return nil
}

// shouter is a second Greeter, so the tests can ask for an ambiguous one
type shouter struct{}

func (s *shouter) Id() string    { return "shouter" }
func (s *shouter) New() any      { return new(shouter) }
func (s *shouter) Greet() string { return "HI" }

type brokenInit struct{}

func (b *brokenInit) Id() string { return "broken-init" }
func (b *brokenInit) New() any   { return new(brokenInit) }
func (b *brokenInit) Initialize(*Context) error {
	return errors.New("no database")
}

type brokenValidate struct{}

func (b *brokenValidate) Id() string { return "broken-validate" }
func (b *brokenValidate) New() any   { return new(brokenValidate) }
func (b *brokenValidate) Validate() error {
	return errors.New("missing setting")
}

func contextWith(modules ...Module) *Context {
	registry := NewRegistry()
	for _, m := range modules {
		registry.Register(m)
	}

	return NewContext(registry, Config{})
}

func TestAModuleIsResolvedByItsConcreteType(t *testing.T) {
	inits := 0
	ctx := contextWith(&greeter{inits: &inits})

	mod, err := GetModule[*greeter](ctx, new(greeter))
	require.NoError(t, err)
	assert.Equal(t, "hi", mod.Greet())
	assert.Equal(t, 1, inits)
}

// Modules are wired against interfaces, which is what lets a test swap one out
func TestAModuleIsResolvedByAnInterfaceItImplements(t *testing.T) {
	ctx := contextWith(&greeter{})

	mod, err := GetModule[Greeter](ctx, new(Greeter))
	require.NoError(t, err)
	assert.Equal(t, "hi", mod.Greet())
}

// Everything asking for the same module has to get the same instance, otherwise
// each caller would open its own database
func TestAModuleIsInitializedOnceAndSharedAfterwards(t *testing.T) {
	inits := 0
	ctx := contextWith(&greeter{inits: &inits})

	first, err := ctx.Module(new(greeter))
	require.NoError(t, err)

	second, err := ctx.Module(new(greeter))
	require.NoError(t, err)

	assert.Same(t, first, second)
	assert.Equal(t, 1, inits, "the module was initialized more than once")
}

func TestAskingForAModuleThatIsNotRegistered(t *testing.T) {
	_, err := contextWith().Module(new(greeter))
	assert.Error(t, err)
}

// Two modules behind one interface is a wiring mistake, not a choice to make
func TestAnAmbiguousInterfaceIsAnError(t *testing.T) {
	ctx := contextWith(&greeter{}, &shouter{})

	_, err := ctx.Module(new(Greeter))
	assert.Error(t, err)

	// Both are still reachable by their own type
	found, err := ctx.Modules(new(Greeter))
	require.NoError(t, err)
	assert.Len(t, found, 2)
}

// Asking for the wrong type is caught instead of panicking on the assertion
func TestGetModuleRefusesTheWrongType(t *testing.T) {
	ctx := contextWith(&greeter{})

	_, err := GetModule[*shouter](ctx, new(greeter))
	assert.Error(t, err)
}

func TestAModuleThatFailsToStartStopsTheChain(t *testing.T) {
	_, err := contextWith(&brokenInit{}).Module(new(brokenInit))

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInitializingModule)
	assert.Contains(t, err.Error(), "broken-init", "the failing module should be named")
	assert.Contains(t, err.Error(), "no database")
}

func TestAModuleThatFailsValidationStopsTheChain(t *testing.T) {
	_, err := contextWith(&brokenValidate{}).Module(new(brokenValidate))

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidatingModule)
	assert.Contains(t, err.Error(), "broken-validate")
}

// Two modules under one id would silently shadow each other, so it is a panic
// at startup rather than a surprise at runtime
func TestRegisteringTheSameIdTwicePanics(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&greeter{})

	assert.PanicsWithError(t, "greeter: "+ErrModuleRegistered.Error(), func() {
		registry.Register(&greeter{})
	})
}

func TestTheRegistryHandsBackEverythingRegistered(t *testing.T) {
	registry := NewRegistry()
	assert.Empty(t, registry.All())

	registry.Register(&greeter{})
	registry.Register(&shouter{})

	ids := make([]string, 0, 2)
	for _, m := range registry.All() {
		ids = append(ids, m.Id())
	}

	assert.ElementsMatch(t, []string{"greeter", "shouter"}, ids)
}
