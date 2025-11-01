package cluster

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/wildmap/utility/app/chanrpc"
)

type IClusterRPC interface {
	Cast(svrKind int64, svrIdx int64, module string, msg any)
	Call(svrKind int64, svrIdx int64, module string, msg any) *chanrpc.RetInfo
	CallWithTimeout(timeout time.Duration, svrKind int64, svrIdx int64, module string, msg any) *chanrpc.RetInfo
	ASyncCall(svrKind int64, svrIdx int64, module string, msg any, cb chanrpc.Callback) error
}

func init() {
	_ = Register((*RPCPackReq)(nil))
	_ = Register((*RPCPackAck)(nil))
}

var (
	global     IClusterRPC
	globalOnce sync.Once
)

func Init(addr string, kind int64, idx int64) (err error) {
	if addr == "" || kind <= 0 || idx <= 0 {
		return fmt.Errorf("parameter is invalid")
	}
	globalOnce.Do(func() {
		global, err = NewNatRPC(addr, kind, idx)
	})

	return
}

func Call(svrKind int64, svrIdx int64, module string, msg any) *chanrpc.RetInfo {
	if global == nil {
		return &chanrpc.RetInfo{
			Err: fmt.Errorf("global IClusterRPC is nil"),
		}
	}
	return global.Call(svrKind, svrIdx, module, msg)
}

func CallWithTimeout(timeout time.Duration, svrKind int64, svrIdx int64, module string, msg any) *chanrpc.RetInfo {
	if global == nil {
		return &chanrpc.RetInfo{
			Err: fmt.Errorf("global IClusterRPC is nil"),
		}
	}
	return global.CallWithTimeout(timeout, svrKind, svrIdx, module, msg)
}

func Cast(svrKind int64, svrIdx int64, module string, msg any) {
	if global == nil {
		slog.Warn(fmt.Sprintf("global IClusterRPC is nil"))
		return
	}
	global.Cast(svrKind, svrIdx, module, msg)
}
