package logic

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/b2network/b2-sign/internal/config"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/psbt"
	"github.com/btcsuite/btcd/txscript"
)

const (
	WaitTimeout    = 1 * time.Minute
	ContextTimeout = 2 * time.Hour
	SignTimeout    = 2 * time.Second
)

type SignService struct {
	cfg    *config.Config
	key    *btcec.PrivateKey
	b2node *NodeClient
}

func NewSignService(
	cfg *config.Config,
	key *btcec.PrivateKey,
	b2node *NodeClient,
) *SignService {
	s := &SignService{cfg, key, b2node}
	return s
}

// Start
func (s *SignService) Start() error {
	ticker := time.NewTicker(WaitTimeout)
	for {
		<-ticker.C
		ticker.Reset(WaitTimeout)
		log.Println("start handle unsigned data")
		err := s.Handle()
		if err != nil {
			log.Println("handle err:", err.Error())
		}
	}
}

func (s *SignService) Handle() error {
	defer func() {
		if err := recover(); err != nil {
			log.Print(err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), ContextTimeout)
	defer cancel()

	unsignedRsp, err := s.b2node.Unsigned(ctx)
	if err != nil {
		return err
	}

	log.Println("unsigned data len:", len(unsignedRsp))
	for _, tx := range unsignedRsp {
		pack, err := psbt.NewFromRawBytes(strings.NewReader(tx.EncodedData), true)
		if err != nil {
			log.Println("decode psbt raw err:", err.Error())
			return err
		}
		signPack, err := s.SignPsbt(pack)
		if err != nil {
			log.Println("sign psbt err:", err.Error())
			return err
		}

		err = s.b2node.Sign(ctx, tx.TxId, signPack)
		if err != nil {
			log.Println("b2node send sign data err:", err.Error())
			return err
		}
		time.Sleep(SignTimeout)
	}
	return nil
}

func (s *SignService) SignPsbt(pack *psbt.Packet) (*psbt.Packet, error) {
	tx := pack.UnsignedTx
	updater, err := psbt.NewUpdater(pack)
	if err != nil {
		return nil, err
	}
	prevOutputFetcher := txscript.NewMultiPrevOutFetcher(nil)
	for i, in := range tx.TxIn {
		prevOutputFetcher.AddPrevOut(in.PreviousOutPoint, pack.Inputs[i].WitnessUtxo)
	}
	for i := range tx.TxIn {
		witnessSig, err := txscript.RawTxInWitnessSignature(
			tx,
			txscript.NewTxSigHashes(tx, prevOutputFetcher),
			i,
			pack.Inputs[i].WitnessUtxo.Value,
			pack.Inputs[i].WitnessScript,
			txscript.SigHashAll,
			s.key,
		)
		if err != nil {
			return nil, err
		}
		pack.Inputs[i].FinalScriptSig = witnessSig
		updater.AddInSighashType(txscript.SigHashAll, i)
	}
	return updater.Upsbt, nil
}
