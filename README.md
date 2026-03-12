# 🛠️ Utility

![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)

---

## 📖 项目简介 (Introduction)

### 🎯 一句话概述

**Utility** 是一个为 荒野地图 开发者打造的**高性能通用工具库**，专注于后端系统的基础设施建设。

### 💡 项目背景与价值

在后端开发过程中，开发者经常需要重复实现诸如 ID 生成、加密解密、网络通信、配置加载等基础功能。这些"轮子"的重复造就不仅浪费时间，还容易引入潜在的 bug 和性能问题。

**Utility** 应运而生，它提供了一套经过**生产环境验证**的、**开箱即用**的核心组件集合，涵盖：

- 🔢 **基础工具层**：数值计算、ID 生成、加密解密、数据结构
- 🌐 **网络通信层**：多协议服务器（TCP/KCP/WebSocket）、连接管理
- 🏗️ **应用框架层**：模块化架构、生命周期管理、事件总线
- 📦 **中间件集成**：RabbitMQ 客户端、日志系统、脚本执行器
- ⚙️ **配置管理层**：TSV 热加载、类型安全解析

### 🎯 主要应用场景

- **游戏服务器开发**：坐标计算、战斗逻辑、状态管理、配置热更新
- **微服务架构**：分布式 ID、消息队列、服务通信、日志追踪
- **高并发系统**：网络连接管理、协议多路复用、优雅关闭
- **数据密集型应用**：加密传输、配置管理、脚本动态执行

---

## ✨ 核心特性 (Features)

### ⚡ 高性能基础库

- **🆔 IDGen (分布式 ID 生成器)**
  - 基于改进的 Snowflake 算法（42 位时间戳 + 21 位序列号）
  - 支持 139 年时间跨度，每秒生成 210 万唯一 ID
  - 线程安全，支持高并发场景
  - 内置时间戳与序列号提取功能

- **🔍 Accuracy (浮点数精度比较)**
  - 解决浮点数 `==` 比较的精度误差问题
  - 提供 `Equal`、`Greater`、`Smaller` 等高级比较方法
  - 支持自定义精度阈值（Epsilon / LowEpsilon）
  - 适用场景：游戏坐标、金融计算、物理模拟

- **🔐 Crypto (企业级加密工具)**
  - **RSA-OAEP**：非对称加密，支持 PKCS#1/PKCS#8 密钥格式
  - **AES-GCM**：认证加密（推荐），提供机密性与完整性双重保护
  - **AES-CTR**：流式加密，适合大数据加密
  - 统一的 `ICrypter` 接口设计，便于算法切换

- **📊 高性能数据结构**
  - **Flag**：64 位标记系统，内存占用仅 8 字节，支持位运算
  - **TopN**：泛型 Top-N 容器，自动维护最大/最小 N 个元素
  - **SortMap**：Map 排序工具，支持按键/值升降序排序

### 🛠️ 强大的模块组件

- **🏗️ App Framework (应用框架)**
  - 模块化架构设计，支持动态模块加载/卸载
  - 完整的生命周期管理（Init → Run → Close）
  - 内置 ChanRPC 通道，支持模块间异步通信
  - 支持模块状态监控与统计

- **🌐 XNet (多协议网络服务器)**
  - **协议支持**：TCP、KCP（可靠 UDP）、WebSocket
  - **连接复用**：基于 `cmux` 的智能协议分流
  - **高并发设计**：Goroutine Pool、连接计数、优雅关闭
  - **安全特性**：支持 HAProxy PROXY 协议、Panic 恢复
  - **跨平台**：支持 Windows 命名管道、Unix Domain Socket

- **📄 TSV Config (配置管理)**
  - 支持热加载（文件变更自动重载）
  - 泛型设计，类型安全的配置访问
  - 支持 JSON 嵌套字段解析
  - 自动类型转换（int/float/bool/string/json）
  - 线程安全读写

- **📢 Event Bus (事件总线)**
  - 轻量级进程内发布/订阅模式
  - 支持监听器优先级排序
  - 快速注册接口（`QuickRegister`）
  - 自动清理空监听集合，防止内存泄漏

- **📝 XLog (高性能日志)**
  - 基于 `Uber Zap` 构建，性能极致
  - 提供 `Sugar` 风格的便捷 API
  - 支持结构化日志与格式化日志
  - 灵活的日志级别控制

- **🐍 XExec (脚本执行器)**
  - 支持 Python、Shell、PowerShell、Batch 脚本
  - 环境变量隔离与工作目录控制
  - 实时输出捕获（stdout/stderr）
  - 超时控制与优雅终止
  - 注入式测试支持（便于单元测试）

- **⏰ XTime (时间工具库)**
  - 时间偏移功能（用于测试/调试时间相关逻辑）
  - 毫秒/秒/微秒级时间戳转换
  - 支持 UTC 与本地时间切换
  - 今日零点、下一日零点等常用时间计算

- **🐰 XAMQP (RabbitMQ 客户端)**
  - 封装 `amqp091-go`，简化 RabbitMQ 操作
  - 自动重连机制，支持连接断线重试
  - 声明式队列/交换机/绑定管理
  - 支持发布/订阅、工作队列等模式

---

## 🚀 使用指南 (Usage)

### 1. 安装依赖

```bash
# 获取最新版本
go get github.com/wildmap/utility@latest

# 或指定版本
go get github.com/wildmap/utility@v1.0.0
```

### 2. 快速开始

#### 🆔 分布式 ID 生成 (IDGen)

```go
package main

import (
	"fmt"
	"github.com/wildmap/utility"
)

func main() {
	// 生成全局唯一 ID
	id := utility.NextID()
	
	fmt.Printf("ID: %d\n", id.Int64())          // 数值形式
	fmt.Printf("String: %s\n", id.String())     // 字符串形式
	fmt.Printf("生成时间: %v\n", id.Time())       // 提取时间戳
	fmt.Printf("序列号: %d\n", id.Seq())          // 提取序列号
	
	// 从字符串解析 ID
	parsed, _ := utility.ParseString("123456789")
	fmt.Printf("解析结果: %d\n", parsed.Int64())
}
```

**输出示例：**
```
ID: 187649984473252865
String: 187649984473252865
生成时间: 2026-02-03 15:17:30 +0800 CST
序列号: 1
```

#### 🔍 浮点数精度比较 (Accuracy)

```go
package main

import (
	"fmt"
	"github.com/wildmap/utility"
)

func main() {
	// 普通比较（错误示范）
	a, b := 0.1+0.2, 0.3
	fmt.Printf("0.1 + 0.2 == 0.3: %v\n", a == b)  // false（精度问题）
	
	// 使用 Accuracy 比较（正确做法）
	fmt.Printf("Equal: %v\n", utility.Equal(a, b))              // true
	fmt.Printf("Greater: %v\n", utility.Greater(1.02, 1.0))     // true
	fmt.Printf("GreaterOrEqual: %v\n", utility.GreaterOrEqual(1.005, 1.0)) // true（近似相等）
}
```

#### 🔐 加密解密 (Crypto)

```go
package main

import (
	"fmt"
	"github.com/wildmap/utility"
)

func main() {
	// 1. AES-GCM 认证加密（推荐用于数据传输）
	key := []byte("1234567890123456") // 16 bytes (AES-128)
	crypter, _ := utility.NewAESGCMCrypter(key)
	
	plaintext := []byte("敏感数据")
	ciphertext, _ := crypter.Encrypt(plaintext)
	decrypted, _ := crypter.Decrypt(ciphertext)
	
	fmt.Printf("原文: %s\n", plaintext)
	fmt.Printf("密文: %x...\n", ciphertext[:16]) // 十六进制显示
	fmt.Printf("解密: %s\n", decrypted)
	
	// 2. RSA 非对称加密（用于密钥交换）
	publicKeyPEM := []byte(`-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...
-----END PUBLIC KEY-----`)
	
	encrypter, _ := utility.NewRSAEncrypter(publicKeyPEM)
	encrypted, _ := encrypter.Encrypt([]byte("secret key"))
	fmt.Printf("RSA 加密: %x...\n", encrypted[:16])
}
```

#### 🏴 位标记管理 (Flag)

```go
package main

import (
	"fmt"
	"github.com/wildmap/utility"
)

// 定义权限标志
const (
	PermRead    utility.Flag = 1 << 0  // 0x01 读权限
	PermWrite   utility.Flag = 1 << 1  // 0x02 写权限
	PermExecute utility.Flag = 1 << 2  // 0x04 执行权限
	PermDelete  utility.Flag = 1 << 3  // 0x08 删除权限
)

func main() {
	var userPerms utility.Flag
	
	// 设置权限
	userPerms.Set(PermRead | PermWrite)
	fmt.Printf("权限值: %08b\n", userPerms)  // 二进制: 00000011
	
	// 检查权限
	fmt.Printf("有读权限: %v\n", userPerms.Include(PermRead))        // true
	fmt.Printf("有写和执行权限: %v\n", userPerms.Include(PermWrite | PermExecute)) // false
	fmt.Printf("有读或执行权限: %v\n", userPerms.IncludeAny(PermRead | PermExecute)) // true
	
	// 清除权限
	userPerms.Clean(PermWrite)
	fmt.Printf("清除写权限后: %08b\n", userPerms)  // 00000001
}
```

#### 📊 Top-N 排序容器

```go
package main

import (
	"fmt"
	"github.com/wildmap/utility"
)

type Player struct {
	Name  string
	Score int
}

func main() {
	// 维护分数最高的 3 名玩家
	topN := utility.NewTopN[Player](3, func(a, b Player) int {
		return a.Score - b.Score  // 按分数升序排列
	})
	
	// 添加玩家
	players := []Player{
		{"Alice", 95}, {"Bob", 87}, {"Charlie", 92},
		{"David", 78}, {"Eve", 99},
	}
	
	for _, p := range players {
		topN.Put(p)
	}
	
	// 遍历前 3 名
	fmt.Println("排行榜 Top 3:")
	topN.Range(func(i int, p Player) bool {
		fmt.Printf("%d. %s: %d 分\n", i+1, p.Name, p.Score)
		return false
	})
}
```

**输出：**
```
排行榜 Top 3:
1. Eve: 99 分
2. Alice: 95 分
3. Charlie: 92 分
```

#### 📄 TSV 配置加载

**配置文件：`config/items.tsv`**
```tsv
ID	Name	Level	Attributes
编号	名称	等级	属性
#int	string	int	json
1001	铁剑	5	{"atk": 50, "def": 10}
1002	钢盾	8	{"atk": 20, "def": 80}
1003	法杖	10	{"atk": 100, "magic": 200}
```

**加载代码：**
```go
package main

import (
	"fmt"
	"github.com/wildmap/utility/tsv"
)

type ItemConf struct {
	ID         int                    `json:"ID"`
	Name       string                 `json:"Name"`
	Level      int                    `json:"Level"`
	Attributes map[string]interface{} `json:"Attributes"`
}

func main() {
	// 加载配置（Key 为 int 类型的 ID）
	conf, err := tsv.New[int, *ItemConf]("./config")
	if err != nil {
		panic(err)
	}
	
	// 获取单个配置
	if item := conf.Get(1001); item != nil {
		fmt.Printf("物品: %s (Lv.%d)\n", item.Name, item.Level)
		fmt.Printf("攻击力: %.0f\n", item.Attributes["atk"])
	}
	
	// 遍历所有配置
	conf.Range(func(id int, item *ItemConf) bool {
		fmt.Printf("[%d] %s\n", id, item.Name)
		return false  // 返回 true 可中断遍历
	})
}
```

#### 📡 网络服务器 (XNet)

```go
package main

import (
	"context"
	"fmt"
	"github.com/wildmap/utility/xlog"
	"github.com/wildmap/utility/xnet"
)

// 实现 IAgent 接口
type GameAgent struct {
	conn xnet.IConn
}

func (a *GameAgent) OnInit(ctx context.Context) error {
	xlog.Infof("客户端连接: %s", a.conn.RemoteAddr())
	return nil
}

func (a *GameAgent) Run(ctx context.Context) {
	for {
		// 读取消息
		msg, err := a.conn.ReadMsg()
		if err != nil {
			xlog.Errorf("读取消息失败: %v", err)
			return
		}
		
		xlog.Infof("收到消息: %s", string(msg))
		
		// 回复消息
		response := []byte(fmt.Sprintf("服务器收到: %s", msg))
		if err := a.conn.WriteMsg(response); err != nil {
			xlog.Errorf("发送消息失败: %v", err)
			return
		}
	}
}

func (a *GameAgent) OnClose(ctx context.Context) {
	xlog.Infof("客户端断开: %s", a.conn.RemoteAddr())
}

func main() {
	// 创建支持 TCP/WebSocket/KCP 的服务器
	server := xnet.NewServer(":8080", func(conn xnet.IConn) xnet.IAgent {
		return &GameAgent{conn: conn}
	})
	
	xlog.Info("启动游戏服务器...")
	if err := server.Start(); err != nil {
		panic(err)
	}
	
	// 优雅关闭（通常在信号处理中调用）
	// server.Stop()
}
```

#### 📢 事件总线 (Event)

```go
package main

import (
	"fmt"
	"github.com/wildmap/utility/event"
)

func main() {
	bus := event.NewFacade()
	
	// 注册监听器（支持优先级）
	bus.QuickRegister("user.login", 10, func(data map[string]any) {
		fmt.Printf("用户登录: %s\n", data["username"])
	})
	
	bus.QuickRegister("user.login", 5, func(data map[string]any) {
		fmt.Println("记录登录日志")  // 优先级低，后执行
	})
	
	// 触发事件
	bus.Fire("user.login", map[string]any{
		"username": "Alice",
		"ip":       "192.168.1.100",
	})
}
```

**输出（按优先级排序）：**
```
用户登录: Alice
记录登录日志
```

#### 🐍 脚本执行 (XExec)

```go
package main

import (
	"fmt"
	"github.com/wildmap/utility/xexec"
)

func main() {
	// 执行 Python 脚本
	script := xexec.NewPythonScript("print('Hello from Python!')")
	output, err := script.Run()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Python 输出: %s\n", output)
	
	// 执行 Shell 命令（跨平台）
	shellScript := xexec.NewShellScript("echo 'Hello World'")
	shellOutput, _ := shellScript.Run()
	fmt.Printf("Shell 输出: %s\n", shellOutput)
}
```

#### ⏰ 时间工具 (XTime)

```go
package main

import (
	"fmt"
	"github.com/wildmap/utility/xtime"
	"time"
)

func main() {
	// 获取当前时间戳
	fmt.Printf("当前毫秒时间戳: %d\n", xtime.NowTs())
	fmt.Printf("当前秒时间戳: %d\n", xtime.NowSecTs())
	
	// 时间戳转换
	ms := xtime.NowTs()
	t := xtime.Ms2Time(ms)
	fmt.Printf("时间对象: %v\n", t)
	
	// 时间偏移（用于测试）
	xtime.SetUseOffset(true)
	xtime.AddOffset(24 * time.Hour)  // 将当前时间推进一天
	fmt.Printf("偏移后时间: %v\n", xtime.Now())
	
	// 今日零点
	todayStart := xtime.TodayStartTs()
	fmt.Printf("今日零点: %s\n", xtime.FormatMs(todayStart))
}
```
