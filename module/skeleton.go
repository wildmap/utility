package module

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/wildmap/utility/module/chanrpc"
	"github.com/wildmap/utility/timer"
)

type Skeleton struct {
	name   string
	timer  timer.Timer
	client *chanrpc.Client
	server *chanrpc.Server
}

func NewSkeleton(ctx context.Context, name string) (*Skeleton, error) {
	s := &Skeleton{
		name:   name,
		server: chanrpc.NewServer(),
		client: chanrpc.NewClient(),
	}
	var err error
	s.timer, err = timer.NewTimer(ctx)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Skeleton) Name() string {
	return s.name
}

func (s *Skeleton) Run(closeSig chan bool) {
	for {
		select {
		case <-closeSig:
			slog.Info(fmt.Sprintf("%s skeleton stop", s.name))
			s.timer.Stop()
			return
		case ri := <-s.client.ChanAsynRet:
			s.client.Cb(ri)
		case ci := <-s.server.ChanCall:
			s.server.Exec(ci)
		}
	}
}

func (s *Skeleton) AfterFunc(expire time.Duration, callback func()) (timer.TimeNoder, error) {
	return s.timer.AfterFunc(expire, callback)
}

func (s *Skeleton) ScheduleFunc(expire time.Duration, callback func()) (timer.TimeNoder, error) {
	return s.timer.ScheduleFunc(expire, callback)
}

func (s *Skeleton) ASyncCall(server *chanrpc.Server, req interface{}, cb chanrpc.Callback) error {
	if err := s.client.Attach(server); err != nil {
		return err
	}
	err := s.client.AsynCall(req, cb)
	return err
}

func (s *Skeleton) ClusterASyncCall(server *chanrpc.Server, req interface{}, srcNodeType string, srcServerID int32, cb chanrpc.Callback) error {
	if err := s.client.Attach(server); err != nil {
		return err
	}
	err := s.client.ClusterAsynCall(req, cb, srcNodeType, srcServerID)
	return err
}

func (s *Skeleton) Cast(server *chanrpc.Server, req interface{}) {
	err := server.Cast(req)
	if err != nil {
		slog.Error(fmt.Sprintf("cast msg: %v failed: %v", req, err))
	}
}

func (s *Skeleton) Call(server *chanrpc.Server, req interface{}) *chanrpc.RetInfo {
	if err := s.client.Attach(server); err != nil {
		slog.Error(fmt.Sprintf("call msg: %v failed: %v", req, err))
		return &chanrpc.RetInfo{
			Err: err,
		}
	}
	return s.client.Call(req)
}

func (s *Skeleton) Call0(server *chanrpc.Server, msg interface{}) *chanrpc.RetInfo {
	return s.Call(server, msg)
}

func (s *Skeleton) RegisterChanRPC(id interface{}, f chanrpc.Handler) {
	s.server.Register(id, f)
}
