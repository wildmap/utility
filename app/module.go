package app

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"slices"
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
	Name() string                // 名称
	Priority() uint              // 模块优先级, 值越小优先级越高
	OnInit() error               // 初始化
	OnStart(ctx context.Context) // 启动, 阻塞
	OnDestroy()                  // 销毁
	ChanRPC() *chanrpc.Server    // 消息通道
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
	defaultShutdownTimeout = 30 * time.Minute
)

// moduleWrapper 使用额外的运行时信息包装模块
type moduleWrapper struct {
	IModule
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// app 中的 modules 在初始化(通过 Start 或 Run) 之后不能变更
// 只有 Get 和 Stats 是 goroutine safe 的
type app struct {
	sync.RWMutex
	modules        []*moduleWrapper
	dynamicModules sync.Map
	state          int32
}

// newApp 创建App
func newApp() *app {
	return &app{
		state:   AppStateNone,
		modules: make([]*moduleWrapper, 0),
	}
}

// setState 设置状态
func (a *app) setState(state int32) {
	atomic.StoreInt32(&a.state, state)
}

// GetState 获取状态
func (a *app) GetState() int32 {
	return atomic.LoadInt32(&a.state)
}

// Stats 获取所有模块状态
func (a *app) Stats() string {
	a.RLock()
	defer a.RUnlock()

	var builder strings.Builder

	// Static modules
	for _, wrapper := range a.modules {
		a.appendModuleStats(&builder, "static", wrapper)
	}

	// Dynamic modules
	a.dynamicModules.Range(func(key, value any) bool {
		if wrapper, ok := value.(*moduleWrapper); ok {
			a.appendModuleStats(&builder, "dynamic", wrapper)
		}
		return true
	})

	return builder.String()
}

// appendModuleStats 添加模块状态
func (a *app) appendModuleStats(builder *strings.Builder, moduleType string, wrapper *moduleWrapper) {
	rpcServer := wrapper.ChanRPC()

	if rpcServer != nil {
		channelLen := len(rpcServer.ChanCall)
		builder.WriteString(fmt.Sprintf("%s: %s, rpc_queue_length: %d\n",
			moduleType, wrapper.Name(), channelLen))
	} else {
		builder.WriteString(fmt.Sprintf("%s: %s, rpc_queue_length: N/A\n",
			moduleType, wrapper.Name()))
	}
}

// GetChanRPC 获取模块的RPC服务
func (a *app) GetChanRPC(name string) *chanrpc.Server {
	// Check static modules first
	a.RLock()
	for _, wrapper := range a.modules {
		if wrapper.Name() == name {
			a.RUnlock()
			return wrapper.ChanRPC()
		}
	}
	a.RUnlock()

	// Check dynamic modules
	return a.getChanRPCDynamic(name)
}

// getChanRPCDynamic 获取动态模块的RPC服务
func (a *app) getChanRPCDynamic(name string) *chanrpc.Server {
	if value, ok := a.dynamicModules.Load(name); ok {
		if wrapper, ok := value.(*moduleWrapper); ok {
			return wrapper.ChanRPC()
		}
	}
	return nil
}

func (a *app) Register(mods ...IModule) error {
	if a.GetState() != AppStateNone {
		return fmt.Errorf("application is already running")
	}

	for _, mod := range mods {
		a.Lock()
		wrapper := &moduleWrapper{
			IModule: mod,
		}
		wrapper.ctx, wrapper.cancel = context.WithCancel(context.Background())
		a.modules = append(a.modules, wrapper)
		a.Unlock()
	}

	return nil
}

// Run 按顺序启动和停止模块，自动监测 SIGINT SIGKILL 信号
func (a *app) Run(mods ...IModule) {
	var errCh = make(chan bool, 1)
	go func() {
		if !a.start(mods...) {
			errCh <- true
		}
	}()

	// Setup signal handling
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Wait for termination signal
	for {
		select {
		case <-errCh:
			xlog.Errorln("application failed to start")
			return
		case sig := <-signalChan:
			xlog.Infof("received shutdown signal %s", sig)

			if sig == syscall.SIGHUP {
				xlog.Infof("SIGHUP received, continuing operation")
				continue
			}
			goto STOP
		}

	}
STOP:
	a.stop()
}

// start 初始化app
func (a *app) start(mods ...IModule) bool {
	defer func() {
		if r := recover(); r != nil {
			xlog.Errorf("application panic recovered, panic %v\n%s", r, string(debug.Stack()))
			os.Exit(255)
		}
	}()
	currentState := a.GetState()
	if currentState != AppStateNone {
		xlog.Errorf("application cannot start twice, current state is %d", currentState)
		return false
	}

	// Register modules
	a.Lock()
	for _, mod := range mods {
		if mod == nil {
			xlog.Warnln("application cannot register nil module")
			continue
		}
		wrapper := &moduleWrapper{
			IModule: mod,
		}
		wrapper.ctx, wrapper.cancel = context.WithCancel(context.Background())
		a.modules = append(a.modules, wrapper)
	}
	a.Unlock()

	if len(a.modules) == 0 {
		xlog.Warnf("no modules provided to start")
		return false
	}

	// 按优先级对模块进行排序（升序排列，值越小优先级越高），优先级相同时按名称排序
	slices.SortStableFunc(a.modules, func(i, j *moduleWrapper) int {
		if n := cmp.Compare(i.Priority(), j.Priority()); n != 0 {
			return n
		}
		return strings.Compare(i.Name(), j.Name())
	})

	a.setState(AppStateInit)
	xlog.Infof("application starting, module count: %d", len(a.modules))
	for _, wrapper := range a.modules {
		xlog.Infof("module startup order %s (priority: %d)", wrapper.Name(), wrapper.Priority())
	}

	// Initialize modules
	for _, wrapper := range a.modules {
		if err := wrapper.OnInit(); err != nil {
			xlog.Errorf("module %s initialization failed, err %v", wrapper.Name(), err)
			return false
		}
	}

	// Start module goroutines
	for _, wrapper := range a.modules {
		wrapper.wg.Add(1)
		go a.onStartModule(wrapper, false)
	}

	a.setState(AppStateRun)
	xlog.Infof("application started successfully")
	return true
}

// onStartModule 启动模块
func (a *app) onStartModule(wrapper *moduleWrapper, dynamic bool) {
	// lock current go routine to a system thread
	runtime.LockOSThread()
	defer func() {
		// unlock current go routine
		runtime.UnlockOSThread()
		// decrement wait group
		wrapper.wg.Done()
		if r := recover(); r != nil {
			xlog.Errorf("module %s panic recovered, panic %v\n%s", wrapper.Name(), r, string(debug.Stack()))
			if !dynamic {
				os.Exit(255)
			}
		}
	}()

	xlog.Infof("started module %s", wrapper.Name())

	wrapper.OnStart(wrapper.ctx)
	xlog.Infof("module %s stopped", wrapper.Name())
}

// stop 停止app
func (a *app) stop() {
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
func (a *app) shutdownModule(wrapper *moduleWrapper) {
	// Signal module to stop
	xlog.Infof("signaling module %s shutdown", wrapper.Name())
	wrapper.cancel()

	// Wait for module with timeout
	done := make(chan struct{})
	go func() {
		wrapper.wg.Wait()
		close(done)
	}()

	timer := time.NewTimer(defaultShutdownTimeout)
	defer timer.Stop()
	select {
	case <-done:
		xlog.Infof("module %s goroutine exited", wrapper.Name())
	case <-timer.C:
		xlog.Errorf("module %s shutdown timeout", wrapper.Name())
	}

	// Destroy module
	xlog.Infof("destroying module %s", wrapper.Name())
	a.destroyModule(wrapper)

	xlog.Infof("module %s shutdown complete", wrapper.Name())
}

// destroyModule 销毁模块
func (a *app) destroyModule(wrapper *moduleWrapper) {
	defer func() {
		if r := recover(); r != nil {
			xlog.Errorf("module %s destroy panic recovered, panic %v\n%s", wrapper.Name(), r, string(debug.Stack()))
		}
	}()

	wrapper.OnDestroy()
}

func (a *app) DynamicModules() (res []string) {
	a.dynamicModules.Range(func(key, value any) bool {
		res = append(res, key.(string))
		return true
	})
	return
}

// AddDynamicModules 添加动态模块
func (a *app) AddDynamicModules(mods ...IModule) error {
	var wrappers []*moduleWrapper
	for _, mod := range mods {
		if mod == nil {
			xlog.Warnln("application cannot register nil module")
			continue
		}
		wrapper := &moduleWrapper{
			IModule: mod,
		}
		wrapper.ctx, wrapper.cancel = context.WithCancel(context.Background())
		wrappers = append(wrappers, wrapper)
	}

	// 按优先级对模块进行排序（升序排列，值越小优先级越高），优先级相同时按名称排序
	slices.SortStableFunc(wrappers, func(i, j *moduleWrapper) int {
		if n := cmp.Compare(i.Priority(), j.Priority()); n != 0 {
			return n
		}
		return cmp.Compare(i.Name(), j.Name())
	})
	for _, wrapper := range wrappers {
		if err := wrapper.OnInit(); err != nil {
			xlog.Errorf("module %s init error %v", wrapper.Name(), err)
			return fmt.Errorf("module %s init failed: %w", wrapper.Name(), err)
		}
		// start module goroutine
		wrapper.wg.Add(1)
		go a.onStartModule(wrapper, true)
		a.dynamicModules.Store(wrapper.Name(), wrapper)
	}
	return nil
}

// RemoveDynamicModule 删除动态模块
func (a *app) RemoveDynamicModule(name string) bool {
	value, ok := a.dynamicModules.Load(name)
	if !ok {
		return false
	}

	wrapper, ok := value.(*moduleWrapper)
	if !ok {
		return false
	}

	// Signal shutdown
	wrapper.cancel()

	// Wait for completion
	wrapper.wg.Wait()

	// Destroy
	a.destroyModule(wrapper)

	// Remove from map
	a.dynamicModules.Delete(name)

	return true
}

// removeAllDynamicModules 删除所有动态模块
func (a *app) removeAllDynamicModules() {
	var moduleNames []string

	// Collect all module names first
	a.dynamicModules.Range(func(key, value any) bool {
		moduleNames = append(moduleNames, key.(string))
		return true
	})

	// Remove each module
	for _, name := range moduleNames {
		a.RemoveDynamicModule(name)
	}
}
