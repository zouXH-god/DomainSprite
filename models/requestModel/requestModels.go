package requestModel

type CreateCertificateRequest struct {
	DomainId     string `form:"domainId" json:"domainId" uri:"domainId"`
	DomainIdList string `form:"domainIdList" json:"domainIdList" uri:"domainIdList"`
}

type GetCertificateListRequest struct {
	Page     int `form:"page" json:"page" default:"0"`
	PageSize int `form:"pageSize" json:"pageSize" default:"10"`
}

type DomainNameListRequest struct {
	DomainNameList string `form:"domainNameList" json:"domainNameList" uri:"domainNameList" binding:"required"`
}

type CertificateIdRequest struct {
	CertificateId int `form:"certificateId" json:"certificateId" uri:"certificateId" binding:"required"`
}

type DownloadCertificateViewWithIdRequest struct {
	CertificateIdRequest
	DownloadType string `form:"downloadType" json:"downloadType" uri:"downloadType" binding:"required"`
}

type TaskIdRequest struct {
	Id int `form:"id" json:"id" uri:"id" binding:"required"`
}
