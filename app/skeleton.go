package app

import (
	"fmt"
	"log/slog"

	"github.com/wildmap/utility/app/chanrpc"
	"github.com/wildmap/utility/app/timermgr"
)

type Skeleton struct {
	name   string
	timer  *timermgr.TimerMgr
	server *chanrpc.Server
	client *chanrpc.Client
}

func NewSkeleton(name string) *Skeleton {
	s := &Skeleton{
		name:   name,
		server: chanrpc.NewServer(),
		client: chanrpc.NewClient(),
		timer:  timermgr.NewTimerMgr(),
	}
	return s
}

func (s *Skeleton) Name() string {
	return s.name
}

func (s *Skeleton) Run(closeSig chan bool) {
	s.timer.Run()
	for {
		select {
		case <-closeSig:
			slog.Info(fmt.Sprintf("%s skeleton stop", s.name))
			s.close()
			return
		case ri := <-s.client.ChanAsynRet:
			s.client.Cb(ri)
		case ci := <-s.server.ChanCall:
			s.server.Exec(ci)
		case t := <-s.timer.ChanTimer():
			t.Cb()
		}
	}
}

func (s *Skeleton) close() {
	s.server.Close()
	slog.Info(fmt.Sprintf("%s skeleton server close", s.name))
	s.timer.Stop()
	slog.Info(fmt.Sprintf("%s skeleton timer close", s.name))
}

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
		slog.Error(fmt.Sprintf("cast msg: %v failed: %v", req, err))
	}
}

func (s *Skeleton) Call(server *chanrpc.Server, req any) *chanrpc.RetInfo {
	if err := s.client.Attach(server); err != nil {
		slog.Error(fmt.Sprintf("call msg: %v failed: %v", req, err))
		return &chanrpc.RetInfo{
			Err: err,
		}
	}
	return s.client.Call(req)
}
