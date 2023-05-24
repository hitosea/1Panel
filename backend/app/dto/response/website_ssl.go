package response

import "1Panel/backend/app/model"

type WebsiteSSLDTO struct {
	model.WebsiteSSL
}

type WebsiteDNSRes struct {
	Key    string `json:"resolve"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
	Err    string `json:"err"`
}

type WebsiteAcmeAccountDTO struct {
	model.WebsiteAcmeAccount
}

type WebsiteDnsAccountDTO struct {
	model.WebsiteDnsAccount
	Authorization map[string]string `json:"authorization"`
}
