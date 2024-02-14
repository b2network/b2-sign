package btc

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
)

func GenerateMultiSigScript(pubs []string, minSignNum int, testnet bool) (string, []byte, error) {
	net := &chaincfg.MainNetParams
	if testnet {
		net = &chaincfg.TestNet3Params
	}
	var allPubKeys []*btcutil.AddressPubKey
	for _, pub := range pubs {
		pubKey, err := hex.DecodeString(pub)
		if err != nil {
			return "", nil, err
		}

		addressPubKey, err := btcutil.NewAddressPubKey(pubKey, net)
		if err != nil {
			return "", nil, err
		}
		allPubKeys = append(allPubKeys, addressPubKey)
	}
	builder := txscript.NewScriptBuilder()
	builder.AddInt64(int64(minSignNum))
	for _, key := range allPubKeys {
		builder.AddData(key.ScriptAddress())
	}
	builder.AddInt64(int64(len(allPubKeys)))
	builder.AddOp(txscript.OP_CHECKMULTISIG)
	script, err := builder.Script()
	if err != nil {
		return "", nil, err
	}
	h256 := sha256.Sum256(script)
	witnessProg := h256[:]
	address, err := btcutil.NewAddressWitnessScriptHash(witnessProg, net)
	if err != nil {
		return "", nil, err
	}
	return address.EncodeAddress(), script, nil
}
