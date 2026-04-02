package registry

import "sync"

func newTestModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		models:               make(map[string]*ModelRegistration),
		clientModels:         make(map[string][]string),
		clientModelInfos:     make(map[string]map[string]*ModelInfo),
		clientProviders:      make(map[string]string),
		availableModelsCache: make(map[string]availableModelsCacheEntry),
		mutex:                &sync.RWMutex{},
	}
}
