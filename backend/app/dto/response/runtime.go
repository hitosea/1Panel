package response

import "1Panel/backend/app/model"

type RuntimeRes struct {
	model.Runtime
	AppParams []AppParam `json:"appParams"`
	AppID     uint       `json:"appId"`
}
