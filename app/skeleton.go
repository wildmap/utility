package app

import (
	"context"
	"fmt"

	"github.com/wildmap/utility/app/chanrpc"
	"github.com/wildmap/utility/app/timermgr"
	"github.com/wildmap/utility/xlog"
)

type IRPC interface {
	Cast(server *chanrpc.Server, req any)
	Call(server *chanrpc.Server, req any) *chanrpc.RetInfo
	ASyncCall(server *chanrpc.Server, req any, cb chanrpc.Callback) error
}

type ITimer interface {
	RegisterTimer(kind string, handler timermgr.TimerHandler)
	NewTimer(duraMs int64, kind string, metadata map[string]string) int64
	NewTicker(duraMs int64, kind string, metadata map[string]string) int64
	AccTimer(id int64, kind timermgr.AccKind, value int64) error
	DelayTimer(id int64, kind timermgr.AccKind, value int64) (err error)
	CancelTimer(id int64)
}

// Skeleton 基础框架
type Skeleton struct {
	name   string
	timer  *timermgr.TimerMgr
	server *chanrpc.Server
	client *chanrpc.Client
}

// NewSkeleton 创建Skeleton, l为定时器数量
func NewSkeleton(name string) *Skeleton {
	s := &Skeleton{
		name:   name,
		server: chanrpc.NewServer(10000),
		client: chanrpc.NewClient(10000),
		timer:  timermgr.NewTimerMgr(10000),
	}
	return s
}

// Name 名称
func (s *Skeleton) Name() string {
	return s.name
}

// Priority 优先级 (值越小优先级越高)
func (s *Skeleton) Priority() uint {
	return 0
}

// OnStart 启动
func (s *Skeleton) OnStart(ctx context.Context) {
	s.timer.Run()
	for {
		select {
		case <-ctx.Done():
			s.close()
			xlog.Infof("%s stopped", s.name)
			return
		case ri := <-s.client.ChanASyncRet:
			s.client.Cb(ri)
		case ci := <-s.server.ChanCall:
			s.server.Exec(ci)
		case t := <-s.timer.ChanTimer():
			t.Cb()
		}
	}
}

// close 关闭
func (s *Skeleton) close() {
	s.timer.Stop()
	s.server.Close()
	for !s.client.Idle() {
		s.client.Close()
		xlog.Infof("%s skeleton client close ", s.Name())
	}
}

func (s *Skeleton) RegisterTimer(kind string, handler timermgr.TimerHandler) {
	s.timer.RegisterTimer(kind, handler)
}

// NewTimer 启动Timer
func (s *Skeleton) NewTimer(duraMs int64, kind string, metadata map[string]string) int64 {
	return s.timer.NewTimer(duraMs, kind, metadata)
}

// NewTicker 启动Ticker，id为0则新建，否则复用id
func (s *Skeleton) NewTicker(id int64, duraMs int64, kind string, metadata map[string]string) int64 {
	return s.timer.NewTicker(id, duraMs, kind, metadata)
}

func (s *Skeleton) AccTimer(id int64, kind timermgr.AccKind, value int64) error {
	return s.timer.AccTimer(id, kind, value)
}

func (s *Skeleton) DelayTimer(id int64, kind timermgr.AccKind, value int64) (err error) {
	return s.timer.DelayTimer(id, kind, value)
}

func (s *Skeleton) UpdateTimer(id int64, endTs int64) {
	s.timer.UpdateTimer(id, endTs)
}

func (s *Skeleton) CancelTimer(id int64) {
	s.timer.CancelTimer(id)
}

// ChanRPC 获取ChanRPC
func (s *Skeleton) ChanRPC() *chanrpc.Server {
	return s.server
}

func (s *Skeleton) RegisterChanRPC(msg any, f chanrpc.Handler) error {
	return s.server.Register(msg, f)
}

func (s *Skeleton) ASyncCall(server *chanrpc.Server, req any, cb chanrpc.Callback) error {
	if server == nil {
		return fmt.Errorf("async call msg: %v failed: %v", req, "server nil")
	}
	if err := s.client.Attach(server); err != nil {
		return err
	}
	err := s.client.ASynCall(req, cb)
	return err
}

func (s *Skeleton) Cast(server *chanrpc.Server, req any) {
	if server == nil {
		xlog.Warnf("cast msg: %v failed: %v", req, "server nil")
		return
	}
	err := server.Cast(req)
	if err != nil {
		xlog.Warnf("cast msg: %v failed: %v", req, err)
	}
	return
}

func (s *Skeleton) Call(server *chanrpc.Server, req any) *chanrpc.RetInfo {
	if server == nil {
		return &chanrpc.RetInfo{
			Err: fmt.Errorf("call msg: %v failed: %v", req, "server nil"),
		}
	}
	if err := s.client.Attach(server); err != nil {
		return &chanrpc.RetInfo{
			Err: err,
		}
	}
	return s.client.Call(req)
}
