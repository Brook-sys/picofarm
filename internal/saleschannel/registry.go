package saleschannel

import (
	"errors"
	"fmt"
)

var (
	// ErrProviderAlreadyRegistered is returned when registering a duplicate channel ID.
	ErrProviderAlreadyRegistered = errors.New("sales channel provider already registered")
	// ErrProviderNotFound is returned when a provider cannot be found by channel ID.
	ErrProviderNotFound = errors.New("sales channel provider not found")
	// ErrInvalidCapability is returned when a descriptor declares an unknown capability.
	ErrInvalidCapability = errors.New("invalid sales channel capability")
)

// Registry stores sales-channel providers by channel ID.
type Registry struct {
	providers map[ChannelID]Provider
	order     []ChannelID
}

// NewRegistry creates an empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[ChannelID]Provider),
	}
}

// Register adds a provider to the registry.
func (r *Registry) Register(provider Provider) error {
	descriptor := provider.Descriptor()
	if descriptor.ID == "" {
		return fmt.Errorf("%w: empty channel id", ErrProviderNotFound)
	}
	for _, capability := range descriptor.Capabilities {
		if !capability.IsValid() {
			return fmt.Errorf("%w: %s", ErrInvalidCapability, capability)
		}
	}
	if _, exists := r.providers[descriptor.ID]; exists {
		return fmt.Errorf("%w: %s", ErrProviderAlreadyRegistered, descriptor.ID)
	}

	r.providers[descriptor.ID] = provider
	r.order = append(r.order, descriptor.ID)
	return nil
}

// Get returns a registered provider by channel ID.
func (r *Registry) Get(id ChannelID) (Provider, error) {
	provider, ok := r.providers[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, id)
	}
	return provider, nil
}

// Descriptors returns provider descriptors in registration order.
func (r *Registry) Descriptors() []ProviderDescriptor {
	descriptors := make([]ProviderDescriptor, 0, len(r.order))
	for _, id := range r.order {
		descriptors = append(descriptors, r.providers[id].Descriptor())
	}
	return descriptors
}
