package tsv

import (
	"fmt"
	"testing"
)

type ResourceItem struct {
	Typ string  `json:"typ"`
	Id  int32   `json:"id"`
	Val float64 `json:"val"`
}

type GatherConfig struct {
	Id          int32          `json:"id,omitempty"`
	Name        string         `json:"name,omitempty"`
	Desc        string         `json:"desc,omitempty"`
	Resource    []ResourceItem `json:"resource,omitempty"`
	Yield       []ResourceItem `json:"yield,omitempty"`
	DestoryTime int32          `json:"destory_time,omitempty"`
	RadiusBlock int32          `json:"radius_block,omitempty"`
	RadiusOccup int32          `json:"radius_occup,omitempty"`
}

func Test(t *testing.T) {
	conf, err := New[int32, GatherConfig]("./test")
	if err != nil {
		t.Error(err)
	}
	fmt.Println(conf.Select(func(line GatherConfig) bool {
		if line.Id == 1001 {
			return true
		}
		return false
	}))
	fmt.Println(conf.Get(1))
}
