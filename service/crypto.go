package service

import (
	"crypto/aes"
	"encoding/base64"
	"encoding/json"
)

const tokenPasswordKey = "qzkj1kjghd=876&*"

func encryptJWPassword(password string) (string, error) {
	plainJSON, err := json.Marshal(password)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher([]byte(tokenPasswordKey))
	if err != nil {
		return "", err
	}

	padded := pkcs7Pad(plainJSON, block.BlockSize())
	encrypted := make([]byte, len(padded))
	for start := 0; start < len(padded); start += block.BlockSize() {
		block.Encrypt(encrypted[start:start+block.BlockSize()], padded[start:start+block.BlockSize()])
	}
	firstBase64 := base64.StdEncoding.EncodeToString(encrypted)
	return base64.StdEncoding.EncodeToString([]byte(firstBase64)), nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	out := make([]byte, len(data)+padding)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(padding)
	}
	return out
}
