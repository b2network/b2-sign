package logic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/b2network/b2-sign/internal/config"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/psbt"
	"github.com/btcsuite/btcd/txscript"
)

const (
	ServiceName = "BitcoinSignService"
	WaitTimeout = 1 * time.Minute
)

type SignService struct {
	cfg *config.Config
	key *btcec.PrivateKey
}

// UnsignedAPIResponse
type UnsignedAPIResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Tx []Tx `json:"tx"`
	} `json:"data"`
}

type Tx struct {
	Psbt string `json:"psbt"`
}

type SignedAPIRequest struct {
	Tx []Tx `json:"tx"`
}

func NewSignService(cfg *config.Config, key *btcec.PrivateKey) *SignService {
	s := &SignService{cfg, key}
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
		log.Println("end")
	}
}

func (s *SignService) Handle() error {
	defer func() {
		if err := recover(); err != nil {
			log.Print(err)
		}
	}()

	unsignedRsp, err := s.UnsignedDATA()
	if err != nil {
		return err
	}

	signedTx := make([]Tx, 0)
	for _, tx := range unsignedRsp.Data.Tx {
		pack, err := psbt.NewFromRawBytes(strings.NewReader(tx.Psbt), true)
		if err != nil {
			log.Println("decode psbt raw err")
			return err
		}
		signPack, err := s.SignPsbt(pack)
		if err != nil {
			return err
		}

		signPackB64, err := signPack.B64Encode()
		if err != nil {
			return err
		}

		signedTx = append(signedTx, Tx{
			Psbt: signPackB64,
		})
	}
	signedReq := &SignedAPIRequest{
		Tx: signedTx,
	}
	if err := s.SendSignedDATA(signedReq); err != nil {
		return err
	}

	return nil
}

func (s *SignService) SignPsbt(pack *psbt.Packet) (*psbt.Packet, error) {
	tx := pack.UnsignedTx
	updater, err := psbt.NewUpdater(pack)
	if err != nil {
		return nil, err
	}
	for i, in := range tx.TxIn {
		prevOutputFetcher := txscript.NewMultiPrevOutFetcher(nil)
		prevOutputFetcher.AddPrevOut(in.PreviousOutPoint, pack.Inputs[i].WitnessUtxo)
		witnessSig, err := txscript.RawTxInWitnessSignature(
			tx,
			txscript.NewTxSigHashes(tx, prevOutputFetcher),
			i,
			pack.Inputs[i].WitnessUtxo.Value,
			pack.Inputs[i].WitnessUtxo.PkScript,
			txscript.SigHashAll,
			s.key,
		)
		if err != nil {
			return nil, err
		}
		pack.Inputs[i].PartialSigs = append(pack.Inputs[i].PartialSigs, &psbt.PartialSig{
			PubKey:    s.key.PubKey().SerializeCompressed(),
			Signature: witnessSig,
		})
		updater.AddInSighashType(txscript.SigHashAll, i)
	}
	return updater.Upsbt, nil
}

// UnsignedDATA get unsigned data
func (s *SignService) UnsignedDATA() (*UnsignedAPIResponse, error) {
	data := UnsignedAPIResponse{}
	resp, err := s.Http("POST", s.cfg.UnsignedAPI, nil)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(resp, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

// SendSignedDATA send signed data
func (s *SignService) SendSignedDATA(req *SignedAPIRequest) error {
	bodyData, err := json.Marshal(req)
	if err != nil {
		return err
	}
	_, err = s.Http("POST", s.cfg.SignedAPI, bytes.NewBuffer(bodyData))
	if err != nil {
		return err
	}
	return nil
}

func (s *SignService) Http(method string, url string, bodyData io.Reader) ([]byte, error) {
	client := &http.Client{}
	req, err := http.NewRequest(method, url, bodyData)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http response status code not ok")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}
