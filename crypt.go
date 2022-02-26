// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU Crypto module

package tru

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"hash"
	"io"
	rnd "math/rand"
)

type crypt struct {
	privateKey *rsa.PrivateKey // RSA private key
	sessionKey                 // Current session key
}

const bitSize = 1024 // 896

// newCrypt create and initialize trudp crypt module
func (tru *Tru) newCrypt() (c *crypt, err error) {
	c = new(crypt)
	if tru.privateKey != nil {
		c.privateKey = tru.privateKey
		return
	}
	c.privateKey, err = GeneratePrivateKey()
	if err != nil {
		return
	}
	tru.privateKey = c.privateKey
	return
}

// GeneratePrivateKey create new RSA private key
func GeneratePrivateKey() (privateKey *rsa.PrivateKey, err error) {
	return rsa.GenerateKey(rand.Reader, bitSize)
}

// xorEncryptDecrypt encrypt or decrypt input data by input key
func (c crypt) xorEncryptDecrypt(input, key []byte) {
	kl := len(key)
	for i := range input {
		input[i] = input[i] ^ key[i%kl]
	}
}

// encryptAES encrypt data using AES
func (c crypt) encryptAES(key []byte, data []byte) (out []byte, err error) {

	//Create a new Cipher Block from the key
	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}

	// Create a new GCM - https://en.wikipedia.org/wiki/Galois/Counter_Mode
	// https://golang.org/pkg/crypto/cipher/#NewGCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return
	}

	// Create a nonce. Nonce should be from GCM
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return
	}

	// Encrypt the data using aesGCM.Seal
	// Since we don't want to save the nonce somewhere else in this case, we
	// add it as a prefix to the encrypted data. The first nonce argument in
	// Seal is the prefix.
	out = aesGCM.Seal(nonce, nonce, data, nil)

	return
}

// decryptAES decrypt data using AES
func (c crypt) decryptAES(key []byte, data []byte) (out []byte, err error) {

	//Create a new Cipher Block from the key
	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}

	//Create a new GCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return
	}

	//Get the nonce size
	nonceSize := aesGCM.NonceSize()

	//Extract the nonce from the encrypted data
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]

	//Decrypt the data
	out, err = aesGCM.Open(nil, nonce, ciphertext, nil)

	return
}

// encryptPacketData encryp packet data with packet key
func (c crypt) encryptPacketData(id int, data []byte) (out []byte, err error) {
	if !c.ison() {
		return data, nil
	}
	l := len(data)
	key := c.packetKey(uint32(id), l)
	if l <= 64 {
		out = make([]byte, len(data))
		copy(out, data)
		c.xorEncryptDecrypt(out, key)
		return
	}
	out, err = c.encryptAES(key, data)

	return
}

// decryptPacketData decrypt packet data with packet key
func (c crypt) decryptPacketData(id int, data []byte) (out []byte, err error) {
	if !c.ison() {
		return data, nil
	}
	l := len(data)
	key := c.packetKey(uint32(id), l)
	if l <= 64 {
		c.xorEncryptDecrypt(data, key)
		return data, nil
	}
	out, err = c.decryptAES(key, data)
	return
}

// encrypt input data by RSA public key
func (c crypt) encrypt(publicKey *rsa.PublicKey, in []byte) (data []byte, err error) {
	const splitby = 62 // 32
	l := len(in)
	e := func(i int) (e int) {
		e = i + splitby
		if e > l {
			e = l
		}
		return
	}
	for i := 0; i < l; i = i + splitby {
		var d []byte
		d, err = rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, in[i:e(i)], nil)
		if err != nil {
			return
		}
		data = append(data, d...)
	}
	return
}

// decrypt input data by RSA private key
func (c crypt) decrypt(in []byte) (data []byte, err error) {
	const splitby = 128 // 112
	l := len(in)
	e := func(i int) (e int) {
		e = i + splitby
		if e > l {
			e = l
		}
		return
	}
	for i := 0; i < l; i = i + splitby {
		var d []byte
		d, err = c.privateKey.Decrypt(nil, in[i:e(i)], &rsa.OAEPOptions{Hash: crypto.SHA256})
		if err != nil {
			return
		}
		data = append(data, d...)
	}
	return
}

// publicKeyToBytes convert public key to bytes
func (c crypt) publicKeyToBytes(key *rsa.PublicKey) (pub []byte, err error) {
	pubASN1, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return
	}
	pub = pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: pubASN1})
	return
}

// bytesToPublicKey convert bytes to public key
func (c crypt) bytesToPublicKey(pub []byte) (key *rsa.PublicKey, err error) {
	block, rest := pem.Decode(pub)
	if block == nil {
		err = fmt.Errorf("can't decode public key %v", rest)
		return
	}
	b := block.Bytes
	ifc, err := x509.ParsePKIXPublicKey(b)
	if err != nil {
		return
	}
	key, ok := ifc.(*rsa.PublicKey)
	if !ok {
		err = errors.New("data is not public key")
	}
	return
}

type sessionKey struct {
	bytes []byte
}

// makeSesionKey create new session key
func (k sessionKey) makeSesionKey() (key []byte) {
	key = k.newSHA256Hash()
	return
}

// setSesionKey load session key from binary slice
func (k *sessionKey) setSesionKey(key []byte) {
	k.bytes = key
}

// ison return true if connection established (and crypt enabled)
// func (c *Channel) ison() bool {
// 	return c.crypt != nil && c.crypt.ison()
// }

// ison return true if crypt enable
func (c crypt) ison() bool {
	return len(c.sessionKey.bytes) > 0
}

func (k sessionKey) packetKey(id uint32, len int) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, id)
	key := append(k.bytes, b...)
	// Hash key depend of data len
	//   md5:    16 byte
	//   sha1:   20 byte
	//   sha256: 32 byte
	//   sha512: 64 byte
	//   sha256: more than 64 byte
	var hash hash.Hash
	switch {
	case len <= 16:
		hash = md5.New()
	case len <= 20:
		hash = sha1.New()
	case len <= 32:
		hash = sha256.New()
	case len <= 64:
		hash = sha512.New()
	default:
		hash = sha256.New() // 32 byte key for AES256 encript/decript
	}
	hash.Write(key)
	return hash.Sum(nil)
}

// newSHA256Hash generates a new SHA256 hash based on
// a random number of characters.
func (k sessionKey) newSHA256Hash(n ...int) []byte {
	numRandomCharacters := 32
	if len(n) > 0 {
		numRandomCharacters = n[0]
	}

	randString := k.randomString(numRandomCharacters)

	hash := sha256.New()
	hash.Write([]byte(randString))

	return hash.Sum(nil)
}

// func (k sessionKey) String() string {
// 	return fmt.Sprintf("%x", k.bytes)
// }

// randomString generates n length random string of printable characters
func (k sessionKey) randomString(n int) string {
	return RandomString(n)
}

var characterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

// randomString generates n length random string of printable characters
func RandomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = characterRunes[rnd.Intn(len(characterRunes))]
	}
	return string(b)
}
