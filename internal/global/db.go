package global

import (
	"sync"

	storedb "github.com/ai4next/superman/internal/store/db"
)

var (
	dbMu       sync.RWMutex
	dbRegistry *storedb.DBRegistry
)

func DBRegistry() (*storedb.DBRegistry, error) {
	dbMu.RLock()
	if dbRegistry != nil {
		defer dbMu.RUnlock()
		return dbRegistry, nil
	}
	dbMu.RUnlock()

	dbMu.Lock()
	defer dbMu.Unlock()
	if dbRegistry != nil {
		return dbRegistry, nil
	}
	registry, err := storedb.NewDBRegistry(storedb.RegistryPaths{
		GlobalDBPath: GlobalDBPath(),
		SupermanPath: StateDBPath(),
		ExpertsDir:   ExpertsDir(),
	})
	if err != nil {
		return nil, err
	}
	dbRegistry = registry
	return dbRegistry, nil
}

func ResetDBRegistry() {
	dbMu.Lock()
	defer dbMu.Unlock()
	dbRegistry = nil
}
