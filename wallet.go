package xginx

import "time"

//钱包处理

type IWallet interface {
	//解密一段时间，时间到达后私钥失效
	Decryption(addr string, pw string, time time.Duration) error
	//加密钱包
	Encryption(addr string, pw string) error
	//根据钱包地址获取私钥
	GetPrivate(addr string) (*PrivateKey, error)
}
