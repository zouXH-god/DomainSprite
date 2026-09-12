//go:build integration

package ali

import (
	"DDNSServer/models"
	"os"
	"testing"
)

var provider *AliDNSClient

func TestMain(m *testing.M) {
	if os.Getenv("DOMAINSprite_ALI_KEY_ID") == "" || os.Getenv("DOMAINSprite_ALI_KEY_SECRET") == "" {
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func Init() {
	id, secret := os.Getenv("DOMAINSprite_ALI_KEY_ID"), os.Getenv("DOMAINSprite_ALI_KEY_SECRET")
	if id == "" || secret == "" {
		return
	}
	provider, _ = NewAliDNSClient(models.Account{Name: "integration", Type: "Ali"}, id, secret)
}
func TestGetDomainList(t *testing.T) {
	Init()
	if provider == nil {
		t.Skip("Ali integration credentials not configured")
	}
	list, err := provider.GetDomainList(models.DomainsSearch{})
	if err != nil {
		println(err.Error())
		return
	}
	for _, domain := range list.Domains {
		println(domain.DomainName)
		println(domain.Id)
	}
}

func TestAliDNSClient_GetRecordList(t *testing.T) {
	Init()
	list, err := provider.GetRecordList(models.DNSSearch{
		DomainName: "2233.chat",
	})
	if err != nil {
		println(err.Error())
		return
	}
	for _, record := range list.Records {
		println(record.DomainName)
		println(record.Id)
		println(record.RecordName)
		println(record.RecordType)
		println(record.RecordContent)
		println(record.Line)
		println(record.Ttl)
		println(record.Status)
	}
}

func TestAliDNSClient_AddRecord(t *testing.T) {
	Init()
	record, err := provider.AddRecord(models.RecordInfo{
		DomainName:    "2233.chat",
		RecordName:    "test1234",
		RecordType:    "A",
		RecordContent: "127.0.0.1",
		Line:          "default",
		Ttl:           600,
	})
	if err != nil {
		println(err.Error())
		return
	}
	println(record.DomainName)
	println(record.Id)
}

func TestAliDNSClient_DeleteRecord(t *testing.T) {
	Init()
	record, err := provider.DeleteRecord("", "1891091318176902144")
	if err != nil {
		println(err.Error())
		return
	}
	println(record.DomainName)
}

func TestAliDNSClient_UpdateRecord(t *testing.T) {
	Init()
	record, err := provider.UpdateRecord(models.RecordInfo{
		Id:            "1891090476614973440",
		DomainName:    "2233.chat",
		RecordName:    "test1234",
		RecordType:    "A",
		RecordContent: "127.0.0.8",
		Line:          "default",
		Ttl:           600,
	})
	if err != nil {
		println(err.Error())
		return
	}
	println(record.DomainName)
}

func TestAliDNSClient_SetRecordStatus(t *testing.T) {
	Init()
	record, err := provider.SetRecordStatus("", "1891091318176902144", "disable")
	if err != nil {
		println(err.Error())
		return
	}
	println(record.DomainName)
}
