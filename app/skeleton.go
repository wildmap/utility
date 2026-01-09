package app

import (
	"github.com/wildmap/utility/app/chanrpc"
	"github.com/wildmap/utility/app/timermgr"
	"github.com/wildmap/utility/xlog"
)

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

// Run 运行
func (s *Skeleton) Run(closeSig chan bool) {
	s.timer.Run()
	for {
		select {
		case <-closeSig:
			xlog.Infof("%s skeleton stop", s.name)
			s.close()
			return
		case ri := <-s.client.ChanASynRet:
			s.client.Cb(ri)
		case ci := <-s.server.ChanCall:
			s.server.Exec(ci)
		case t := <-s.timer.ChanTimer():
			t.Cb()
		}
	}
}

func (s *Skeleton) PostRun() {
	xlog.Infof("%s skeleton post run", s.name)
}

// close 关闭
func (s *Skeleton) close() {
	s.server.Close()
	xlog.Infof("%s skeleton server close", s.name)
	s.timer.Stop()
	xlog.Infof("%s skeleton timer close", s.name)
}

// ChanRPC 获取ChanRPC
func (s *Skeleton) ChanRPC() *chanrpc.Server {
	return s.server
}

func (s *Skeleton) RegisterTimer(kind string, handler timermgr.TimerHandler) {
	s.timer.RegisterTimer(kind, handler)
}

// NewTimer 启动Timer，timerID为0则新建TimerID，否则复用TimerID
func (s *Skeleton) NewTimer(duraMs int64, kind string, metadata map[string]string) int64 {
	return s.timer.NewTimer(duraMs, kind, metadata)
}

func (s *Skeleton) NewTicker(duraMs int64, kind string, metadata map[string]string) int64 {
	return s.timer.NewTicker(duraMs, kind, metadata)
}

func (s *Skeleton) AccTimer(id int64, kind timermgr.AccKind, value int64) error {
	return s.timer.AccTimer(id, kind, value)
}

func (s *Skeleton) DelayTimer(id int64, kind timermgr.AccKind, value int64) (err error) {
	return s.timer.DelayTimer(id, kind, value)
}

func (s *Skeleton) CancelTimer(id int64) {
	s.timer.CancelTimer(id)
}

func (s *Skeleton) RegisterChanRPC(msg any, f chanrpc.Handler) error {
	return s.server.Register(msg, f)
}

func (s *Skeleton) ASyncCall(server *chanrpc.Server, req any, cb chanrpc.Callback) error {
	if err := s.client.Attach(server); err != nil {
		return err
	}
	err := s.client.ASynCall(req, cb)
	return err
}

func (s *Skeleton) Cast(server *chanrpc.Server, req any) {
	err := server.Cast(req)
	if err != nil {
		xlog.Errorf("cast msg: %v failed: %v", req, err)
	}
}

func (s *Skeleton) Call(server *chanrpc.Server, req any) *chanrpc.RetInfo {
	if err := s.client.Attach(server); err != nil {
		xlog.Errorf("call msg: %v failed: %v", req, err)
		return &chanrpc.RetInfo{
			Err: err,
		}
	}
	return s.client.Call(req)
}
