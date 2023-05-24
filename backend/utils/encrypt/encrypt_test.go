package encrypt

import (
	"fmt"
	"testing"

	"1Panel/backend/init/viper"
)

func TestStringEncrypt(t *testing.T) {
	viper.Init()
	p, err := StringEncrypt("1Panel@2022")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(p)
}

func TestStringDecrypt(t *testing.T) {
	viper.Init()
	p, err := StringDecrypt("dXn5bVtea+KVLDrLJlpnPIJNfW8TAMmqX1QNMdSGp88=")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(p)
}
