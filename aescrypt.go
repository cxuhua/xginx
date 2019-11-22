package xginx

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

// AES加密
func AesEncrypt(block cipher.Block, data []byte) ([]byte, error) {
	if block == nil {
		return nil, errors.New("block nil")
	}
	//随机生成iv
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	dl := len(data)
	l := (dl/aes.BlockSize)*aes.BlockSize + aes.BlockSize
	if dl%aes.BlockSize == 0 {
		l = dl
	}
	//add iv length
	dd := make([]byte, l+aes.BlockSize)
	n := l - dl
	//copy iv to dd
	copy(dd[0:], iv)
	//copy data to dd
	copy(dd[aes.BlockSize:], data)
	//fill end bytes
	for i := 0; i < n; i++ {
		dd[dl+i+aes.BlockSize] = byte(n)
	}
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(dd[aes.BlockSize:], dd[aes.BlockSize:])
	return dd, nil
}

//检测最后几个字节是否是加密
func bytesEquInt(data []byte, n byte) bool {
	l := len(data)
	if l == 0 {
		return false
	}
	for i := 0; i < l; i++ {
		if data[i] != n {
			return false
		}
	}
	return true
}

// AES解密
func AesDecrypt(block cipher.Block, data []byte) ([]byte, error) {
	if block == nil {
		return nil, errors.New("block nil")
	}
	bys := len(data)
	if bys < 32 || bys%aes.BlockSize != 0 {
		return nil, errors.New("decrypt data length error")
	}
	//16 bytes iv
	iv := data[:aes.BlockSize]
	dd := data[aes.BlockSize:]
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(dd, dd)
	l := len(dd)
	if n := dd[l-1]; n <= aes.BlockSize {
		x := l - int(n)
		if bytesEquInt(dd[x:], n) {
			dd = dd[:x]
		}
	}
	return dd, nil
}

//整理key为 16 24 or 32
func TrimAESKey(key []byte) ([]byte, error) {
	size := len(key) / 8
	if size <= 2 {
		size = 2
	}
	if size > 4 {
		size = 4
	}
	iLen := size * 8
	ikey := make([]byte, iLen)
	if len(key) > iLen {
		copy(ikey[0:], key[:iLen])
	} else {
		copy(ikey[0:], key)
	}
	return ikey, nil
}

//创建加密算法
func NewAESCipher(key []byte) (cipher.Block, error) {
	ikey, err := TrimAESKey(key)
	if err != nil {
		return nil, err
	}
	return aes.NewCipher(ikey)
}
