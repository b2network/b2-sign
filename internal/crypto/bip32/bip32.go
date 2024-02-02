package bip32

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	bip32 "github.com/tyler-smith/go-bip32"
)

type Key struct {
	*bip32.Key
}

func NewMasterKey(seed []byte) (*Key, error) {
	k, err := bip32.NewMasterKey(seed)
	if err != nil {
		return nil, err
	}
	return &Key{k}, nil
}

func (key *Key) NewChildKeyByPathString(childPath string) (*Key, error) {
	arr := strings.Split(childPath, "/")
	currentKey := key.Key
	for _, part := range arr {
		if part == "m" {
			continue
		}

		var harden = false
		if strings.HasSuffix(part, "'") {
			harden = true
			part = strings.TrimSuffix(part, "'")
		}

		id, err := strconv.ParseUint(part, 10, 31)
		if err != nil {
			return nil, err
		}

		var uid = uint32(id)
		if harden {
			uid |= bip32.FirstHardenedChild
		}

		newKey, err := currentKey.NewChildKey(uid)
		if err != nil {
			return nil, err
		}
		currentKey = newKey
	}
	return &Key{currentKey}, nil
}

func (key *Key) ECPrivKey() (*btcec.PrivateKey, error) {
	if !key.IsPrivate {
		return nil, fmt.Errorf("no private key")
	}

	privKey, _ := btcec.PrivKeyFromBytes(key.Key.Key)
	return privKey, nil
}
