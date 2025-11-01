package app

import (
	"github.com/wildmap/utility/app/chanrpc"
)

// defaultApp is the singleton instance of the application module manager.
var defaultApp = newApp()

// Run initializes and starts all provided modules using the default app instance.
// It blocks until the application receives a termination signal.
func Run(mods ...IModule) {
	defaultApp.Run(mods...)
}

// GetState returns the current state of the default application.
// Thread-safe operation.
func GetState() int32 {
	return defaultApp.GetState()
}

// Stats returns statistics information about all modules.
func Stats() string {
	return defaultApp.Stats()
}

// GetChanRPC retrieves the RPC server for a specific module by name.
// Returns nil if no module with the given name exists.
func GetChanRPC(name string) *chanrpc.Server {
	return defaultApp.GetChanRPC(name)
}

// AddDynamicModules adds dynamic modules to the application.
func AddDynamicModules(mods ...IModule) error {
	return defaultApp.AddDynamicModules(mods...)
}

// RunDynamicModule starts a dynamic module by name.
func RunDynamicModule(name string) bool {
	return defaultApp.RunDynamicModule(name)
}

// RemoveDynamicModule removes and destroys a dynamic module.
func RemoveDynamicModule(name string) bool {
	return defaultApp.RemoveDynamicModule(name)
}
