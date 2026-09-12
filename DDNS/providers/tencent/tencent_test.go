//go:build integration

package tencent

import (
	"DDNSServer/models"
	"os"
	"testing"
)

var provider *TencentDNSClient

func TestMain(m *testing.M) {
	if os.Getenv("DOMAINSprite_TENCENT_SECRET_ID") == "" || os.Getenv("DOMAINSprite_TENCENT_SECRET_KEY") == "" {
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func Init() {
	id, secret := os.Getenv("DOMAINSprite_TENCENT_SECRET_ID"), os.Getenv("DOMAINSprite_TENCENT_SECRET_KEY")
	if id == "" || secret == "" {
		return
	}
	provider, _ = NewTencentProvider(models.Account{Name: "integration", Type: "Tencent"}, id, secret)
}

func TestTencentDNSClient_GetDomainList(t *testing.T) {
	Init()
	if provider == nil {
		t.Skip("Tencent integration credentials not configured")
	}
	info := models.DomainsSearch{
		PageNumber: 1,
		PageSize:   10,
	}
	result, _ := provider.GetDomainList(info)
	t.Log(result)
}

func TestTencentDNSClient_GetRecordList(t *testing.T) {
	Init()
	info := models.DNSSearch{
		DomainName: "s1f.asia",
		PageNumber: 1,
		PageSize:   10,
	}
	result, _ := provider.GetRecordList(info)
	t.Log(result)
}

func TestTencentDNSClient_AddRecord(t *testing.T) {
	Init()
	info := models.RecordInfo{
		DomainName:    "s1f.asia",
		RecordName:    "test2",
		RecordType:    "A",
		RecordContent: "127.0.0.1",
		Ttl:           600,
		Weight:        1,
		Line:          "默认",
	}
	result, _ := provider.AddRecord(info)
	t.Log(result)
}

func TestTencentDNSClient_UpdateRecord(t *testing.T) {
	Init()
	info := models.RecordInfo{
		Id:            "1970336269",
		DomainName:    "s1f.asia",
		RecordName:    "test2",
		RecordType:    "A",
		RecordContent: "127.0.0.8",
		Ttl:           600,
		Weight:        1,
		Line:          "默认",
	}
	result, _ := provider.UpdateRecord(info)
	t.Log(result)
}

func TestTencentDNSClient_DeleteRecord(t *testing.T) {
	Init()
	info := models.RecordInfo{
		Id:         "1970336269",
		DomainName: "s1f.asia",
	}
	result, _ := provider.DeleteRecord(info.DomainName, info.Id)
	t.Log(result)
}

func TestTencentDNSClient_SetRecordStatus(t *testing.T) {
	Init()
	info := models.RecordInfo{
		Id:         "1970336270",
		DomainName: "s1f.asia",
		Status:     "ENABLE",
	}
	result, _ := provider.SetRecordStatus(info.DomainName, info.Id, info.Status)
	t.Log(result)
}
