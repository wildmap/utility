package app

import (
	"fmt"
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
	"github.com/wildmap/utility/xlog"
)

// IModule 模块接口
type IModule interface {
	Name() string             // 名称
	OnInit() error            // 初始化
	Run(closeSig chan bool)   // 运行
	OnDestroy()               // 销毁
	ChanRPC() *chanrpc.Server //消息通道
}

// 节点全局状态
const (
	AppStateNone = iota // 未开始或已停止
	AppStateInit        // 正在初始化中
	AppStateRun         // 正在运行中
	AppStateStop        // 正在停止中
)

const (
	// 默认关闭超时时间
	defaultShutdownTimeout = 3 * time.Minute
	// 默认关闭信号缓冲区大小
	closeSigBufferSize = 1
)

// moduleWrapper 使用额外的运行时信息包装模块
type moduleWrapper struct {
	module   IModule
	closeSig chan bool
	wg       sync.WaitGroup
}

// App 中的 modules 在初始化(通过 Start 或 Run) 之后不能变更
// 只有 Get 和 Stats 是 goroutine safe 的
type App struct {
	sync.RWMutex
	modules        []*moduleWrapper
	dynamicModules sync.Map
	state          int32
}

// NewApp 创建App
func NewApp() *App {
	return &App{
		state:   AppStateNone,
		modules: make([]*moduleWrapper, 0),
	}
}

// setState 设置状态
func (a *App) setState(state int32) {
	atomic.StoreInt32(&a.state, state)
}

// GetState 获取状态
func (a *App) GetState() int32 {
	return atomic.LoadInt32(&a.state)
}

// Stats 获取所有模块状态
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

// appendModuleStats 添加模块状态
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

// GetChanRPC 获取模块的RPC服务
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

// getChanRPCDynamic 获取动态模块的RPC服务
func (a *App) getChanRPCDynamic(name string) *chanrpc.Server {
	if value, ok := a.dynamicModules.Load(name); ok {
		if wrapper, ok := value.(*moduleWrapper); ok {
			return wrapper.module.ChanRPC()
		}
	}
	return nil
}

func (a *App) Register(mod IModule) error {
	if a.GetState() != AppStateNone {
		xlog.Errorf("application is already running, cannot add module %s", mod.Name())
		return fmt.Errorf("application is already running")
	}

	a.Lock()
	a.modules = append(a.modules, &moduleWrapper{
		module:   mod,
		closeSig: make(chan bool, closeSigBufferSize),
	})
	a.Unlock()
	return nil
}

// Run 按顺序启动和停止模块，自动监测 SIGINT SIGKILL 信号
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
		xlog.Infof("received shutdown signal %s", sig)

		if sig == syscall.SIGHUP {
			xlog.Infof("SIGHUP received, continuing operation")
			continue
		}

		break
	}

	a.stop()
}

// start 初始化app
func (a *App) start(mods ...IModule) bool {
	currentState := a.GetState()
	if currentState != AppStateNone {
		xlog.Errorf("application cannot start twice, current state is %d", currentState)
		return false
	}

	// Register modules
	a.Lock()
	for _, mod := range mods {
		a.modules = append(a.modules, &moduleWrapper{
			module:   mod,
			closeSig: make(chan bool, closeSigBufferSize),
		})
	}
	a.Unlock()

	if len(a.modules) == 0 {
		xlog.Warnf("no modules provided to start")
		return false
	}

	a.setState(AppStateInit)
	xlog.Infof("application starting, module count: %d", len(a.modules))

	// Initialize modules
	for _, wrapper := range a.modules {
		moduleName := wrapper.module.Name()
		xlog.Infof("initializing module %s", moduleName)

		if err := wrapper.module.OnInit(); err != nil {
			xlog.Errorf("module %s initialization failed, type %v, err %v", moduleName, reflect.TypeOf(wrapper.module), err)
			return false
		}

		xlog.Infof("initialized successfully module %s", moduleName)
	}

	// Start module goroutines
	for _, wrapper := range a.modules {
		wrapper.wg.Add(1)
		go a.runModule(wrapper)
	}

	a.setState(AppStateRun)
	xlog.Infof("application started successfully")
	return true
}

// runModule 启动模块
func (a *App) runModule(wrapper *moduleWrapper) {
	defer func() {
		wrapper.wg.Done()
		if r := recover(); r != nil {
			xlog.Errorf("module %s panic recovered, moudle, panic %v\n%s", wrapper.module.Name(), r, string(debug.Stack()))
		}
	}()

	moduleName := wrapper.module.Name()
	xlog.Infof("running module %s", moduleName)
	wrapper.module.Run(wrapper.closeSig)
	xlog.Infof("module %s stopped", moduleName)
}

// stop 停止App
func (a *App) stop() {
	if a.GetState() == AppStateStop {
		xlog.Warnf("application already stopping")
		return
	}

	a.setState(AppStateStop)
	xlog.Infof("application shutdown initiated")

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
	xlog.Infof("application shutdown complete")
}

// shutdownModule 关闭模块
func (a *App) shutdownModule(wrapper *moduleWrapper) {
	moduleName := wrapper.module.Name()

	// Signal module to stop
	xlog.Infof("signaling module %s shutdown", moduleName)
	select {
	case wrapper.closeSig <- true:
	default:
		xlog.Warnf("module %s close signal channel full", moduleName)
	}

	// Wait for module with timeout
	done := make(chan struct{})
	go func() {
		wrapper.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		xlog.Infof("module %s goroutine exited", moduleName)
	case <-time.After(defaultShutdownTimeout):
		xlog.Errorf("module %s shutdown timeout", moduleName)
	}

	// Destroy module
	xlog.Infof("destroying module %s", moduleName)
	a.destroyModule(wrapper)

	xlog.Infof("module %s shutdown complete", moduleName)
}

// destroyModule 销毁模块
func (a *App) destroyModule(wrapper *moduleWrapper) {
	defer func() {
		if r := recover(); r != nil {
			xlog.Errorf("module %s destroy panic recovered, panic %v\n%s", wrapper.module.Name(), r, string(debug.Stack()))
		}
	}()

	wrapper.module.OnDestroy()
}

// AddDynamicModules 添加动态模块
func (a *App) AddDynamicModules(mods ...IModule) error {
	for _, mod := range mods {
		wrapper := &moduleWrapper{
			module:   mod,
			closeSig: make(chan bool, closeSigBufferSize),
		}

		if err := mod.OnInit(); err != nil {
			xlog.Errorf("module %v init error %v", reflect.TypeOf(mod), err)
			return fmt.Errorf("module %s init failed: %w", mod.Name(), err)
		}

		a.dynamicModules.Store(mod.Name(), wrapper)
		xlog.Infof("dynamic module %s added", mod.Name())
	}
	return nil
}

// RunDynamicModule 运行动态模块
func (a *App) RunDynamicModule(name string) bool {
	value, ok := a.dynamicModules.Load(name)
	if !ok {
		xlog.Warnf("dynamic module %s not found", name)
		return false
	}

	wrapper := value.(*moduleWrapper)
	wrapper.wg.Add(1)
	go a.runModule(wrapper)

	xlog.Infof("dynamic module %s started", name)
	return true
}

// RemoveDynamicModule 删除动态模块
func (a *App) RemoveDynamicModule(name string) bool {
	value, ok := a.dynamicModules.Load(name)
	if !ok {
		return false
	}

	wrapper := value.(*moduleWrapper)

	xlog.Infof("removing dynamic %s module", name)

	// Signal shutdown
	select {
	case wrapper.closeSig <- true:
	default:
		xlog.Warnf("dynamic module %s close signal channel full", name)
	}

	// Wait for completion
	wrapper.wg.Wait()

	// Destroy
	xlog.Infof("destroying dynamic %s module", name)
	a.destroyModule(wrapper)

	// Remove from map
	a.dynamicModules.Delete(name)

	xlog.Infof("dynamic module %s removed", name)

	return true
}

// removeAllDynamicModules 删除所有动态模块
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
