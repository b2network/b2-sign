package logic_test

import (
	"crypto/sha256"
	"testing"

	"github.com/b2network/b2-sign/internal/config"
	"github.com/b2network/b2-sign/internal/logic"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/psbt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

func TestSign(t *testing.T) {
	// gen 2-3 multi script, simulate actual use
	pk1, err := btcec.NewPrivateKey()
	require.NoError(t, err)
	pk2, err := btcec.NewPrivateKey()
	require.NoError(t, err)
	pk3, err := btcec.NewPrivateKey()
	require.NoError(t, err)
	allPubkeys := [][]byte{}
	allPubkeys = append(allPubkeys, pk1.PubKey().SerializeCompressed(), pk2.PubKey().SerializeCompressed(), pk3.PubKey().SerializeCompressed())
	_, pkScript, err := generateMultiPkScript(allPubkeys, 2, &chaincfg.MainNetParams)
	require.NoError(t, err)
	// create simulate tx
	tx := wire.NewMsgTx(wire.TxVersion)
	hash, err := chainhash.NewHashFromStr("00")
	require.NoError(t, err)
	tx.TxIn = append(tx.TxIn, &wire.TxIn{
		PreviousOutPoint: wire.OutPoint{
			Hash: *hash,
		},
	})
	tx.TxOut = append(tx.TxOut, wire.NewTxOut(2, pkScript))
	unsignedPack := createPsbtFromTx(t, tx)
	for i := range tx.TxIn {
		input := psbt.NewPsbtInput(nil, wire.NewTxOut(2, pkScript))
		unsignedPack.Inputs[i] = *input
	}
	s := mockSignService(t, pk1)
	signPack, err := s.SignPsbt(unsignedPack)
	require.NoError(t, err)
	for _, p := range signPack.Inputs {
		if len(p.PartialSigs) == 0 {
			t.Error("no sig")
		}
	}
}

func createPsbtFromTx(t *testing.T, tx *wire.MsgTx) *psbt.Packet {
	tx2 := tx.Copy()
	unsignedPsbt, err := psbt.NewFromUnsignedTx(tx2)
	require.NoError(t, err)
	return unsignedPsbt
}

func mockSignService(t *testing.T, key *btcec.PrivateKey) *logic.SignService {
	cfg := config.Config{}
	s := logic.NewSignService(&cfg, key)
	return s
}

func generateMultiPkScript(pubKeys [][]byte, minSignNum int, net *chaincfg.Params) (string, []byte, error) {
	var allPubKeys []*btcutil.AddressPubKey
	for _, pubKey := range pubKeys {
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

	witnessScript, err := builder.Script()
	if err != nil {
		return "", nil, err
	}
	h256 := sha256.Sum256(witnessScript)
	witnessProg := h256[:]
	address, err := btcutil.NewAddressWitnessScriptHash(witnessProg, net)
	if err != nil {
		return "", nil, err
	}
	return address.EncodeAddress(), witnessScript, nil
}
