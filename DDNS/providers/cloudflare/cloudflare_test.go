package cloudflare

import (
	"DDNSServer/models"
	"testing"
)

func TestUse(t *testing.T) {

	provider, err := NewCloudflareProvider("2222222222222222", "3333333333333@gmail.com")
	if err != nil {
		return
	}
	// 获取域名列表
	list, err := provider.GetDomainList(models.DomainsSearch{})
	if err != nil {
		return
	}
	for _, domain := range list.Domains {
		println(domain.DomainName)
		println(domain.Id)

		// 添加解析
		//info := models.RecordInfo{
		//	Id:            "",
		//	DomainId:      domain.Id,
		//	DomainName:    domain.DomainName,
		//	RecordName:    "test3",
		//	RecordType:    "A",
		//	RecordContent: "127.0.0.1",
		//}
		//record, err := provider.AddRecord(info)
		//if err != nil {
		//	println(err.Error())
		//	return
		//}

		// 获取解析列表
		records, err := provider.GetRecordList(models.DNSSearch{
			DomainId: domain.Id,
		})
		if err != nil {
			println(err.Error())
			return
		}
		recordOne := models.RecordInfo{}
		for _, record := range records {
			println(record.Id)
			println(record.RecordName)
			recordOne = record
		}

		// 删除解析
		//recordId := "3195a009b98d1fe5a98538d2e74ac124"
		//record, err := provider.DeleteRecord(recordId)
		//if err != nil {
		//	println(err.Error())
		//	return
		//}

		// 修改解析状态
		//record, err = provider.SetRecordStatus(record.Id, "deactivate")
		//if err != nil {
		//	println(err.Error())
		//	return
		//}

		// 修改解析
		recordOne.RecordContent = "114.51.4.12"
		recordOne, err = provider.UpdateRecord(recordOne)
		if err != nil {
			println(err.Error())
			return
		}
		println(recordOne.Id)
	}
}
