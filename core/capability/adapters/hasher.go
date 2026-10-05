package adapters

import (
	"github.com/coolqoo/better-apigate/core/capability"
	"github.com/coolqoo/better-apigate/ports"
)

// HasherAdapter wraps a ports.Hasher to implement capability.HasherProvider.
type HasherAdapter struct {
	name  string
	inner ports.Hasher
}

// WrapHasher creates a capability.HasherProvider from a ports.Hasher.
func WrapHasher(name string, inner ports.Hasher) *HasherAdapter {
	return &HasherAdapter{name: name, inner: inner}
}

func (a *HasherAdapter) Name() string {
	return a.name
}

func (a *HasherAdapter) Hash(plaintext string) ([]byte, error) {
	return a.inner.Hash(plaintext)
}

func (a *HasherAdapter) Compare(hash []byte, plaintext string) bool {
	return a.inner.Compare(hash, plaintext)
}

// Ensure HasherAdapter implements capability.HasherProvider
var _ capability.HasherProvider = (*HasherAdapter)(nil)
