package jscrypt

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"hash"
	"math/rand"
	"strconv"

	"github.com/zatxm/fhblade"
)

type CryptoJsData struct {
	Ct string `json:"ct"`
	Iv string `json:"iv"`
	S  string `json:"s"`
}

func GenerateBw(bt int64) string {
	return strconv.FormatInt(bt-(bt%21600), 10)
}

func GenerateN(t int64) string {
	timestamp := strconv.FormatInt(t, 10)
	return base64.StdEncoding.EncodeToString([]byte(timestamp))
}

// 解密
func Decrypt(data string, password string) (string, error) {
	encBytes, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", err
	}
	var encData CryptoJsData
	err = fhblade.Json.Unmarshal(encBytes, &encData)
	if err != nil {
		return "", err
	}
	cipherBytes, err := base64.StdEncoding.DecodeString(encData.Ct)
	if err != nil {
		return "", err
	}
	salt, err := hex.DecodeString(encData.S)
	if err != nil {
		return "", err
	}
	dstBytes := make([]byte, len(cipherBytes))
	key, _, err := DefaultEvpKDF([]byte(password), salt)
	iv, _ := hex.DecodeString(encData.Iv)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(dstBytes, cipherBytes)
	result := PKCS5UnPadding(dstBytes)
	return string(result), nil
}

// 加密
func Encrypt(data string, password string) (string, error) {
	salt := make([]byte, 8)
	_, err := rand.Read(salt)
	if err != nil {
		return "", err
	}
	key, iv, err := DefaultEvpKDF([]byte(password), salt)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	mode := cipher.NewCBCEncrypter(block, iv)
	cipherBytes := PKCS5Padding([]byte(data), aes.BlockSize)
	mode.CryptBlocks(cipherBytes, cipherBytes)
	md5Hash := md5.New()
	salted := ""
	var dx []byte
	for i := 0; i < 3; i++ {
		md5Hash.Write(dx)
		md5Hash.Write([]byte(password))
		md5Hash.Write(salt)
		dx = md5Hash.Sum(nil)
		md5Hash.Reset()
		salted += hex.EncodeToString(dx)
	}
	cipherText := base64.StdEncoding.EncodeToString(cipherBytes)
	encData := &CryptoJsData{
		Ct: cipherText,
		Iv: salted[64 : 64+32],
		S:  hex.EncodeToString(salt),
	}
	encDataJson, err := fhblade.Json.Marshal(encData)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(encDataJson), nil
}

func PKCS5UnPadding(src []byte) []byte {
	length := len(src)
	unpadding := int(src[length-1])
	return src[:(length - unpadding)]
}

func PKCS5Padding(src []byte, blockSize int) []byte {
	padding := blockSize - len(src)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(src, padtext...)
}

func DefaultEvpKDF(password []byte, salt []byte) (key []byte, iv []byte, err error) {
	keySize := 256 / 32
	ivSize := 128 / 32
	derivedKeyBytes, err := EvpKDF(password, salt, keySize+ivSize, 1, "md5")
	if err != nil {
		return []byte{}, []byte{}, err
	}
	return derivedKeyBytes[:keySize*4], derivedKeyBytes[keySize*4:], nil
}

func EvpKDF(password []byte, salt []byte, keySize int, iterations int, hashAlgorithm string) ([]byte, error) {
	var block []byte
	var hasher hash.Hash
	derivedKeyBytes := make([]byte, 0)
	switch hashAlgorithm {
	case "md5":
		hasher = md5.New()
	default:
		return []byte{}, errors.New("not implement hasher algorithm")
	}
	for len(derivedKeyBytes) < keySize*4 {
		if len(block) > 0 {
			hasher.Write(block)
		}
		hasher.Write(password)
		hasher.Write(salt)
		block = hasher.Sum([]byte{})
		hasher.Reset()

		for i := 1; i < iterations; i++ {
			hasher.Write(block)
			block = hasher.Sum([]byte{})
			hasher.Reset()
		}
		derivedKeyBytes = append(derivedKeyBytes, block...)
	}
	return derivedKeyBytes[:keySize*4], nil
}
