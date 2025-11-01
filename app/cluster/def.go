package cluster

import (
	"fmt"
)

const (
	// svrSubjectFmt GameID.RegionID.AreaID.KindID.InstanceID
	svrSubjectFmt = "%s.%s.%s.%d.%d"
)

func GetSubject(gameID string, regionID string, areaID string, svrKind int64, svrIdx int64) string {
	return fmt.Sprintf(svrSubjectFmt, gameID, regionID, areaID, svrKind, svrIdx)
}

type RPCPackKind int32

const (
	REQ  RPCPackKind = 0
	CAST RPCPackKind = 1
)

type RPCPackReq struct {
	Reply     string      `json:"reply,omitempty"`     // 回复主题
	RpcModule string      `json:"rpcModule,omitempty"` // 处理消息的模块名称(即module的name)
	Kind      RPCPackKind `json:"kind,omitempty"`      // 类型
	SessionID int64       `json:"SessionID,omitempty"` // 会话id
	Data      []byte      `json:"data,omitempty"`      // 数据内容
}

type RPCPackAck struct {
	From      string `json:"from,omitempty"`      // 源主题
	SessionID int64  `json:"SessionID,omitempty"` // 会话id
	Data      []byte `json:"data,omitempty"`      // 数据内容
	Error     string `json:"error,omitempty"`     // 错误
}
