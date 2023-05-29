package nginx

import (
	"1Panel/backend/utils/nginx/components"
	"1Panel/backend/utils/nginx/parser"
)

func GetConfig(path string) (*components.Config, error) {
	p, err := parser.NewParser(path)
	if err != nil {
		return nil, err
	}
	return p.Parse(), nil
}
