// Copyright 2024 The kflashback Authors
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"fmt"
	"sync"
)

// BackendFactory is a function that creates a new Store given a connection string / path.
type BackendFactory func(dsn string) (Store, error)

var (
	mu       sync.RWMutex
	backends = make(map[string]BackendFactory)
)

// RegisterBackend registers a storage backend factory under the given name.
// Typically called from an init() function in the backend package.
func RegisterBackend(name string, factory BackendFactory) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := backends[name]; exists {
		panic(fmt.Sprintf("storage backend %q already registered", name))
	}
	backends[name] = factory
}

// NewStore creates a Store using the named backend and the provided DSN.
func NewStore(backend, dsn string) (Store, error) {
	mu.RLock()
	factory, ok := backends[backend]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown storage backend %q (registered: %v)", backend, RegisteredBackends())
	}
	return factory(dsn)
}

// RegisteredBackends returns the names of all registered storage backends.
func RegisteredBackends() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(backends))
	for name := range backends {
		names = append(names, name)
	}
	return names
}
