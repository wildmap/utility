package app

import (
	"github.com/wildmap/utility/app/chanrpc"
)

// defaultApp 默认单实例
var defaultApp = NewApp()

// Run 默认单实例, 运行应用程序
func Run(mods ...IModule) {
	defaultApp.Run(mods...)
}

// GetState 默认单实例, 获取应用程序状态
func GetState() int32 {
	return defaultApp.GetState()
}

// Stats 默认单实例, 所有模块状态
func Stats() string {
	return defaultApp.Stats()
}

// GetChanRPC 默认单实例, 消息通道
func GetChanRPC(name string) *chanrpc.Server {
	return defaultApp.GetChanRPC(name)
}

// AddDynamicModules 默认单实例, 添加动态模块
func AddDynamicModules(mods ...IModule) error {
	return defaultApp.AddDynamicModules(mods...)
}

// RunDynamicModule 默认单实例, 启动动一个动态模块
func RunDynamicModule(name string) bool {
	return defaultApp.RunDynamicModule(name)
}

// RemoveDynamicModule 默认单实例, 删除动一个动态模块
func RemoveDynamicModule(name string) bool {
	return defaultApp.RemoveDynamicModule(name)
}
