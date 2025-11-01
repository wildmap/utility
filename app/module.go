package app

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"reflect"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/wildmap/utility/app/chanrpc"
)

// IModule defines the interface that all application modules must implement.
type IModule interface {
	Name() string
	OnInit() error
	Run(closeSig chan bool)
	OnDestroy()
	ChanRPC() *chanrpc.Server
}

// Application lifecycle states
const (
	AppStateNone = iota
	AppStateInit
	AppStateRun
	AppStateStop
)

const (
	defaultShutdownTimeout = 30 * time.Second
	closeSigBufferSize     = 1
)

// moduleWrapper wraps a Module with additional runtime information
type moduleWrapper struct {
	module   IModule
	closeSig chan bool
	wg       sync.WaitGroup
}

// App manages the lifecycle of multiple modules in an application.
type App struct {
	sync.RWMutex
	modules        []*moduleWrapper
	dynamicModules sync.Map
	state          int32
}

// newApp creates and returns a new application instance.
func newApp() *App {
	return &App{
		state:     AppStateNone,
		modules:   make([]*moduleWrapper, 0),
	}
}

// setState atomically updates the application state.
func (a *App) setState(state int32) {
	atomic.StoreInt32(&a.state, state)
}

// GetState atomically retrieves the current application state.
func (a *App) GetState() int32 {
	return atomic.LoadInt32(&a.state)
}

// Stats generates a statistics report for all modules using strings.Builder for efficiency.
func (a *App) Stats() string {
	a.RLock()
	defer a.RUnlock()

	var builder strings.Builder

	// Static modules
	for _, wrapper := range a.modules {
		a.appendModuleStats(&builder, "module", wrapper)
	}

	// Dynamic modules
	a.dynamicModules.Range(func(key, value interface{}) bool {
		if wrapper, ok := value.(*moduleWrapper); ok {
			a.appendModuleStats(&builder, "dynamic module", wrapper)
		}
		return true
	})

	return builder.String()
}

// appendModuleStats is a helper to append module statistics
func (a *App) appendModuleStats(builder *strings.Builder, moduleType string, wrapper *moduleWrapper) {
	rpcServer := wrapper.module.ChanRPC()
	moduleName := wrapper.module.Name()

	if rpcServer != nil {
		channelLen := len(rpcServer.ChanCall)
		builder.WriteString(fmt.Sprintf("%s: %s, rpc_queue_length: %d\n",
			moduleType, moduleName, channelLen))
	} else {
		builder.WriteString(fmt.Sprintf("%s: %s, rpc_queue_length: N/A\n",
			moduleType, moduleName))
	}
}

// GetChanRPC retrieves the RPC server for a specific module by name.
func (a *App) GetChanRPC(name string) *chanrpc.Server {
	// Check static modules first
	a.RLock()
	for _, wrapper := range a.modules {
		if wrapper.module.Name() == name {
			a.RUnlock()
			return wrapper.module.ChanRPC()
		}
	}
	a.RUnlock()

	// Check dynamic modules
	return a.getChanRPCDynamic(name)
}

// getChanRPCDynamic retrieves RPC server from dynamic modules
func (a *App) getChanRPCDynamic(name string) *chanrpc.Server {
	if value, ok := a.dynamicModules.Load(name); ok {
		if wrapper, ok := value.(*moduleWrapper); ok {
			return wrapper.module.ChanRPC()
		}
	}
	return nil
}

// Run initializes and starts all provided modules.
func (a *App) Run(mods ...IModule) {
	if !a.start(mods...) {
		return
	}

	// Setup signal handling
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Wait for termination signal
	for {
		sig := <-signalChan
		slog.Info(fmt.Sprintf("received shutdown signal %s", sig))

		if sig == syscall.SIGHUP {
			slog.Info(fmt.Sprintf("SIGHUP received, continuing operation"))
			continue
		}

		break
	}

	a.stop()
}

// start initializes and launches all modules.
func (a *App) start(mods ...IModule) bool {
	currentState := a.GetState()
	if currentState != AppStateNone {
		slog.Error(fmt.Sprintf("application cannot start twice, current state is %d", currentState))
		return false
	}

	if len(mods) == 0 {
		slog.Warn(fmt.Sprintf("no modules provided to start"))
		return false
	}

	// Register modules
	a.Lock()
	a.modules = make([]*moduleWrapper, 0, len(mods))
	for _, mod := range mods {
		a.modules = append(a.modules, &moduleWrapper{
			module:   mod,
			closeSig: make(chan bool, closeSigBufferSize),
		})
	}
	a.Unlock()

	a.setState(AppStateInit)
	slog.Info(fmt.Sprintf("application starting, module count: %d", len(mods)))

	// Initialize modules
	for _, wrapper := range a.modules {
		moduleName := wrapper.module.Name()
		slog.Info(fmt.Sprintf("initializing module %s", moduleName))

		if err := wrapper.module.OnInit(); err != nil {
			slog.Error(fmt.Sprintf("module initialization failed, moudle %s, type %v, err %v", moduleName, reflect.TypeOf(wrapper.module), err))
			return false
		}

		slog.Info(fmt.Sprintf("initialized successfully module %s", moduleName))
	}

	// Start module goroutines
	for _, wrapper := range a.modules {
		wrapper.wg.Add(1)
		go a.runModule(wrapper)
	}

	a.setState(AppStateRun)
	slog.Info(fmt.Sprintf("application started successfully"))
	return true
}

// runModule executes a single module's main loop with panic recovery.
func (a *App) runModule(wrapper *moduleWrapper) {
	defer func() {
		wrapper.wg.Done()
		if r := recover(); r != nil {
			slog.Error(fmt.Sprintf("module panic recovered, moudle %s, panic %v\n%s", wrapper.module.Name(), r, string(debug.Stack())))
		}
	}()

	moduleName := wrapper.module.Name()
	slog.Info(fmt.Sprintf("running module %s", moduleName))
	wrapper.module.Run(wrapper.closeSig)
	slog.Info(fmt.Sprintf("stopped %s", moduleName))
}

// stop gracefully shuts down all modules in reverse order.
func (a *App) stop() {
	if a.GetState() == AppStateStop {
		slog.Warn(fmt.Sprintf("application already stopping"))
		return
	}

	shutdownStart := time.Now()
	a.setState(AppStateStop)
	slog.Info(fmt.Sprintf("application shutdown initiated"))

	// Remove all dynamic modules first
	a.removeAllDynamicModules()

	// Shutdown static modules in reverse order
	a.RLock()
	moduleCount := len(a.modules)
	a.RUnlock()

	for i := moduleCount - 1; i >= 0; i-- {
		a.shutdownModule(a.modules[i])
	}

	a.setState(AppStateNone)
	slog.Info(fmt.Sprintf("application shutdown complete, total_duration_ms %d", time.Since(shutdownStart).Milliseconds()))
}

// shutdownModule gracefully shuts down a single module.
func (a *App) shutdownModule(wrapper *moduleWrapper) {
	moduleName := wrapper.module.Name()
	moduleStart := time.Now()

	// Signal module to stop
	slog.Info(fmt.Sprintf("signaling module shutdown %s", moduleName))
	select {
	case wrapper.closeSig <- true:
	default:
		slog.Warn(fmt.Sprintf("module close signal channel full %s", moduleName))
	}

	// Wait for module with timeout
	done := make(chan struct{})
	go func() {
		wrapper.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info(fmt.Sprintf("module goroutine exited %s", moduleName))
	case <-time.After(defaultShutdownTimeout):
		slog.Error(fmt.Sprintf("module shutdown timeout, moudle %s, timeout %s", moduleName, defaultShutdownTimeout))
	}

	// Destroy module
	slog.Info(fmt.Sprintf("destroying module %s", moduleName))
	a.destroyModule(wrapper)

	slog.Info(fmt.Sprintf("module shutdown complete, module %s, duration_ms %d", moduleName, time.Since(moduleStart).Milliseconds()))
}

// destroyModule safely calls a module's OnDestroy method.
func (a *App) destroyModule(wrapper *moduleWrapper) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error(fmt.Sprintf("module destroy panic recovered %s, panic %v\n%s", wrapper.module.Name(), r,string(debug.Stack())))
		}
	}()

	wrapper.module.OnDestroy()
}

// AddDynamicModules adds and initializes dynamic modules.
func (a *App) AddDynamicModules(mods ...IModule) error {
	for _, mod := range mods {
		wrapper := &moduleWrapper{
			module:   mod,
			closeSig: make(chan bool, closeSigBufferSize),
		}

		if err := mod.OnInit(); err != nil {
			slog.Error(fmt.Sprintf("module %v init error %v", reflect.TypeOf(mod), err))
			return fmt.Errorf("module %s init failed: %w", mod.Name(), err)
		}

		a.dynamicModules.Store(mod.Name(), wrapper)
		slog.Info(fmt.Sprintf("dynamic module added %s", mod.Name()))
	}
	return nil
}

// RunDynamicModule starts a dynamic module by name.
func (a *App) RunDynamicModule(name string) bool {
	value, ok := a.dynamicModules.Load(name)
	if !ok {
		slog.Warn(fmt.Sprintf("dynamic module not found %s", name))
		return false
	}

	wrapper := value.(*moduleWrapper)
	wrapper.wg.Add(1)
	go a.runModule(wrapper)

	slog.Info(fmt.Sprintf("dynamic module started %s", name))
	return true
}

// RemoveDynamicModule removes and destroys a dynamic module.
func (a *App) RemoveDynamicModule(name string) bool {
	value, ok := a.dynamicModules.Load(name)
	if !ok {
		return false
	}

	wrapper := value.(*moduleWrapper)
	moduleStart := time.Now()

	slog.Info(fmt.Sprintf("removing dynamic module %s", name))

	// Signal shutdown
	select {
	case wrapper.closeSig <- true:
	default:
		slog.Warn(fmt.Sprintf("dynamic module close signal channel full %s", name))
	}

	// Wait for completion
	wrapper.wg.Wait()

	// Destroy
	slog.Info(fmt.Sprintf("destroying dynamic module %s", name))
	a.destroyModule(wrapper)

	// Remove from map
	a.dynamicModules.Delete(name)

	slog.Info(fmt.Sprintf("dynamic module removed %s, duration_ms %d", name, time.Since(moduleStart).Milliseconds()))

	return true
}

// removeAllDynamicModules removes all dynamic modules during shutdown.
func (a *App) removeAllDynamicModules() {
	var moduleNames []string

	// Collect all module names first
	a.dynamicModules.Range(func(key, value interface{}) bool {
		moduleNames = append(moduleNames, key.(string))
		return true
	})

	// Remove each module
	for _, name := range moduleNames {
		a.RemoveDynamicModule(name)
	}
}
