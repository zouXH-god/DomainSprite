package models

import (
	"strconv"
)

func (info *RecordInfo) ToTencent() {
	info.IdTC, _ = strconv.ParseUint(info.Id, 10, 64)
	info.DomainIdTC, _ = strconv.ParseUint(info.DomainId, 10, 64)
	info.TtlTC = uint64(info.Ttl)
	info.WeightTC = uint64(info.Weight)
}

type TencentDomainInfo struct {
	DomainInfo
	DomainId uint64 `Tencent:"DomainId"`
}

type TencentDomainsSearch struct {
	DomainsSearch
	GroupId uint64 `Tencent:"GroupId"`
}
