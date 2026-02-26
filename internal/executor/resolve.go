package executor

import "github.com/oseaitic/harbor/internal/cloudauth"

// Resolve returns the appropriate Executor based on login state.
//
// Selection logic (cloud-native, local-capable):
//  1. forceLocal == true → LocalExecutor
//  2. Cloud credentials exist → RemoteExecutor
//  3. Fallback → LocalExecutor
func Resolve(forceLocal bool) Executor {
	if forceLocal {
		return NewLocalExecutor()
	}

	cfg, err := cloudauth.Load()
	if err == nil {
		return NewRemoteExecutor(cfg.Endpoint, cfg.APIKey)
	}

	return NewLocalExecutor()
}
