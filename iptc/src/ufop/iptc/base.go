package iptc

import (
	"encoding/json"
)

type IptcResp struct {
	Width  int      `json:"Width"`
	Height int      `json:"Height"`
	IPTC   IptcInfo `json:"IPTC"`
}

func (x *IptcResp) ToJsonString() string {
	b, _ := json.Marshal(x)
	return string(b)
}

type IptcInfo struct {
	City               string   `json:"City"`
	ObjectName         string   `json:"ObjectName"`
	Keywords           []string `json:"Keywords"`
	OriginatingProgram string   `json:"OriginatingProgram"`
	DateCreated        string   `json:"DateCreated"`
	TimeCreated        string   `json:"TimeCreated"`
}

func (x *IptcInfo) ToJsonString() string {
	b, _ := json.Marshal(x)
	return string(b)
}

////////////////
//目前只支持这几个字段的修改
type IptcReq struct {
	City               string   `json:"City"`
	ObjectName         string   `json:"ObjectName"`
	Keywords           []string `json:"Keywords"`
	OriginatingProgram string   `json:"OriginatingProgram"`
}
