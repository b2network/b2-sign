package bip32_test

import (
	"crypto/rand"
	"testing"

	bip32 "github.com/b2network/b2-sign/internal/crypto/bip32"
	"github.com/stretchr/testify/require"
	"github.com/tyler-smith/go-bip39"
)

func TestKey(t *testing.T) {
	length := 24
	seed := make([]byte, length)
	_, err := rand.Read(seed)
	require.NoError(t, err)
	mnemonic, err := bip39.NewMnemonic(seed)
	require.NoError(t, err)
	bip39Seed := bip39.NewSeed(mnemonic, "")
	masterKey, err := bip32.NewMasterKey(bip39Seed)
	require.NoError(t, err)
	_, err = masterKey.NewChildKeyByPathString("m/48'/1'/0'/2'/0/1/0/0")
	require.NoError(t, err)
}
