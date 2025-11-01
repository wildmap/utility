package cluster

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/wildmap/utility/app"
	"github.com/wildmap/utility/app/chanrpc"
	"github.com/wildmap/utility/queue"
)

type NatsRPC struct {
	addr       string
	gameID     string
	regionId   string
	areaID     string
	svrKind    int64
	svrIdx     int64
	sessionID  int64
	conn       *nats.Conn
	subChan    chan *nats.Msg
	subs       []*nats.Subscription
	queue      queue.IQueue[int64, *chanrpc.RetInfo]
	closeQueue chan int64
}

func NewNatRPC(addr string, kind int64, idx int64) (*NatsRPC, error) {
	ns := &NatsRPC{
		addr:       addr,
		svrKind:    kind,
		svrIdx:     idx,
		subChan:    make(chan *nats.Msg, 1024),
		queue:      queue.NewQueue[int64, *chanrpc.RetInfo](),
		closeQueue: make(chan int64, 1024),
		sessionID:  0,
	}
	ns.gameID = ns.getEnv("GAME_ID", "0")
	ns.regionId = ns.getEnv("REGION_ID", "0")
	ns.areaID = ns.getEnv("AREA_ID", "0")

	conn, err := ns.setupNatsConn(
		ns.addr,
		nats.MaxReconnects(-1),
		nats.Timeout(3*time.Second),
		nats.PingInterval(2*time.Second),
		nats.MaxPingsOutstanding(10),
	)
	if err != nil {
		return nil, err
	}
	ns.conn = conn
	return ns, ns.init()
}

func (r *NatsRPC) Name() string {
	return "natsrpc"
}

func (r *NatsRPC) Conn() *nats.Conn {
	return r.conn
}

func (r *NatsRPC) getEnv(key, fallback string) string {
	val, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	return val
}

func (r *NatsRPC) getSelfSubject(svrKind int64, svrIdx int64) string {
	return GetSubject(r.gameID, r.regionId, r.areaID, svrKind, svrIdx)
}

func (r *NatsRPC) init() error {
	for _, idx := range []int64{0, r.svrIdx} {
		slog.Info(fmt.Sprintf("subscribe %s", r.getSelfSubject(r.svrKind, idx)))
		sub, err := r.subscribe(r.getSelfSubject(r.svrKind, idx))
		if err != nil {
			return err
		}
		r.subs = append(r.subs, sub)
	}
	go func() {
		for {
			select {
			case id := <-r.closeQueue:
				r.queue.Unsubscribe(id)
			case msg := <-r.subChan:
				go r.handlerNatsMsg(msg.Data)
			}
		}
	}()
	return nil
}

func (r *NatsRPC) Cancel() {
	for _, sub := range r.subs {
		_ = sub.Unsubscribe()
	}
}

func (r *NatsRPC) Close() {
	r.Cancel()
	r.conn.Close()
}

func (r *NatsRPC) subscribe(topic string) (*nats.Subscription, error) {
	// 使用nats的组特性实现消息负载均衡
	return r.conn.ChanQueueSubscribe(topic, topic, r.subChan)
}

func (r *NatsRPC) send(svrKind int64, svrIdx int64, module string, msg any) (int64, error) {
	r.sessionID++
	req := &RPCPackReq{
		Kind:      REQ,
		Reply:     r.getSelfSubject(r.svrKind, r.svrIdx),
		RpcModule: module,
		SessionID: r.sessionID,
	}
	data, err := MarshalPack(msg)
	if err != nil {
		return 0, err
	}
	req.Data = data
	pack, err := MarshalPack(req)
	if err != nil {
		return 0, err
	}
	err = r.conn.Publish(r.getSelfSubject(svrKind, svrIdx), pack)
	return r.sessionID, err
}

// Cast 直接投递消息 忽略任何错误和返回值
func (r *NatsRPC) cast(svrKind int64, svrIdx int64, module string, msg any) (int64, error) {
	r.sessionID++
	req := &RPCPackReq{
		Kind:      CAST,
		RpcModule: module,
		SessionID: r.sessionID,
	}
	data, err := MarshalPack(msg)
	if err != nil {
		return 0, err
	}
	req.Data = data
	pack, err := MarshalPack(req)
	if err != nil {
		return 0, err
	}
	err = r.conn.Publish(r.getSelfSubject(svrKind, svrIdx), pack)
	return 0, err
}

func (r *NatsRPC) reply(subj string, sessionID int64, msg any) error {
	if msg == nil {
		return nil
	}
	reply := &RPCPackAck{
		SessionID: sessionID,
		From:      r.getSelfSubject(r.svrKind, r.svrIdx),
	}
	if er, ok := msg.(error); ok {
		reply.Error = er.Error()
	} else {
		data, err := MarshalPack(msg)
		if err != nil {
			return err
		}
		reply.Data = data
	}

	pack, err := MarshalPack(reply)
	if err != nil {
		return err
	}
	err = r.conn.Publish(subj, pack)
	return err

}
func (r *NatsRPC) setupNatsConn(connectString string, options ...nats.Option) (*nats.Conn, error) {
	natsOptions := append(
		options,
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			slog.Info(fmt.Sprintf("disconnected from nats!"))
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			slog.Info(fmt.Sprintf("reconnected to nats server %s with address %s in cluster %s!", nc.ConnectedServerName(), nc.ConnectedAddr(), nc.ConnectedClusterName()))
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			err := nc.LastError()
			if err == nil {
				slog.Error(fmt.Sprintf("nats connection closed with no error."))
				return
			}

			slog.Info(fmt.Sprintf("nats connection closed. reason: %q", nc.LastError()))
		}),
	)

	nc, err := nats.Connect(connectString, natsOptions...)
	if err != nil {
		return nil, err
	}
	slog.Info(fmt.Sprintf("connect %s nats ok", connectString))
	return nc, nil
}

func (r *NatsRPC) handlerNatsMsg(msg []byte) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error(fmt.Sprintf("nats message handler panic recovered %s, panic %v\n%s", msg, r, string(debug.Stack())))
		}
	}()
	pack, err := UnmarshalPack(msg)
	if err != nil {
		slog.Error(fmt.Sprintf("nats msg unmarshal pack error %v", err))
		return
	}
	req, ok := pack.(*RPCPackReq)
	if ok {
		r.handlerNatsReq(req)
		return
	}
	ack, ok := pack.(*RPCPackAck)
	if ok {
		r.handlerNatsAck(ack)
	}
}

func (r *NatsRPC) handlerNatsReq(req *RPCPackReq) {
	data, err := UnmarshalPack(req.Data)
	if err != nil {
		_ = r.reply(req.Reply, req.SessionID, err)
		return
	}

	handle := app.GetChanRPC(req.RpcModule)
	if handle == nil {
		slog.Error(fmt.Sprintf("nats msg handle is nil"))
		_ = r.reply(req.Reply, req.SessionID, fmt.Errorf("not find handle %s", req.RpcModule))
		return
	}
	if req.Kind == REQ {
		res := app.GetChanRPC(req.RpcModule).Call(data)
		if res.Err != nil {
			err = r.reply(req.Reply, req.SessionID, res.Err)
		} else {
			err = r.reply(req.Reply, req.SessionID, res.Ack)
		}
		if err != nil {
			slog.Error(fmt.Sprintf("nats msg call %s error %v", req.RpcModule, err))
			return
		}
	}
	if req.Kind == CAST {
		err = app.GetChanRPC(req.RpcModule).Cast(data)
		if err != nil {
			slog.Error(fmt.Sprintf("nats msg cast %s error %v", req.RpcModule, err))
			return
		}
	}
}

func (r *NatsRPC) handlerNatsAck(ack *RPCPackAck) {
	if ack.Error != "" {
		r.queue.Publish(ack.SessionID, &chanrpc.RetInfo{
			Err: fmt.Errorf("%s", ack.Error),
		})
		return
	}
	data, err := UnmarshalPack(ack.Data)
	if err != nil {
		r.queue.Publish(ack.SessionID, &chanrpc.RetInfo{
			Err: fmt.Errorf("%s", ack.Error),
		})
		return
	}
	r.queue.Publish(ack.SessionID, &chanrpc.RetInfo{
		Ack: data,
	})
}

func (r *NatsRPC) call(ctx context.Context, svrKind int64, svrIdx int64, module string, msg any) *chanrpc.RetInfo {
	sid, err := r.send(svrKind, svrIdx, module, msg)
	if err != nil {
		slog.Error(fmt.Sprintf("nats NatsRPC send %s error %v", module, err))
		return &chanrpc.RetInfo{
			Err: fmt.Errorf("nats NatsRPC send %s error %v", module, err),
		}
	}
	_ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		done = make(chan bool)
		res  = new(chanrpc.RetInfo)
	)
	var once sync.Once
	r.queue.Subscribe(_ctx, sid, func(data *chanrpc.RetInfo) {
		once.Do(func() {
			res = data
			done <- true
		})
		r.closeQueue <- sid
	})
	select {
	case <-done:
		return res
	case <-ctx.Done():
		res.Err = fmt.Errorf("nats NatsRPC call %s error %v", module, ctx.Err())
		return res
	}
}

func (r *NatsRPC) ASyncCall(svrKind int64, svrIdx int64, module string, msg any, cb chanrpc.Callback) error {
	sid, err := r.send(svrKind, svrIdx, module, msg)
	if err != nil {
		slog.Error(fmt.Sprintf("nats NatsRPC send %s error %v", module, err))
		return fmt.Errorf("nats NatsRPC send %s error %v", module, err)
	}
	var once sync.Once
	r.queue.Subscribe(context.Background(), sid, func(data *chanrpc.RetInfo) {
		once.Do(func() {
			cb(data)
		})
		r.closeQueue <- sid
	})
	return nil
}

func (r *NatsRPC) Call(svrKind int64, svrIdx int64, module string, msg any) *chanrpc.RetInfo {
	return r.call(context.Background(), svrKind, svrIdx, module, msg)
}

func (r *NatsRPC) CallWithTimeout(timeout time.Duration, svrKind int64, svrIdx int64, module string, msg any) *chanrpc.RetInfo {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return r.call(ctx, svrKind, svrIdx, module, msg)
}

func (r *NatsRPC) Cast(svrKind int64, svrIdx int64, module string, msg any) {
	_, err := r.cast(svrKind, svrIdx, module, msg)
	if err != nil {
		slog.Error(fmt.Sprintf("nats msg cast %s error %v", module, err))
		return
	}
}
