package logic

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"

	"github.com/b2network/b2-sign/internal/config"
	"github.com/btcsuite/btcd/btcutil/psbt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	sdkmath "cosmossdk.io/math"
	clientTx "github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/simapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	xauthsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authTypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/evmos/ethermint/crypto/ethsecp256k1"
	eTypes "github.com/evmos/ethermint/types"
	bridgeTypes "github.com/evmos/ethermint/x/bridge/types"
	feeTypes "github.com/evmos/ethermint/x/feemarket/types"
)

const (
	DefaultBaseGasPrice = 10_000_000
)

type NodeClient struct {
	PrivateKey      ethsecp256k1.PrivKey
	ChainID         string
	GrpcHost        string
	GrpcPort        uint32
	CoinDenom       string
	GasPrices       uint64
	AddressPrefix   string
	B2NodeAddress   string
	UnsignedTxLimit uint64
	GrpcConn        *grpc.ClientConn
}

type Sign struct {
	TxInIndex int
	Sign      []byte
}

func NewNodeClient(
	cfg *config.Config,
	b2NodePrivKey []byte,
	prefix string,
) (*NodeClient, error) {
	pk := ethsecp256k1.PrivKey{
		Key: b2NodePrivKey,
	}

	b2NodeAddress, err := b2NodeAddress(pk, prefix)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", cfg.B2NodeGRPCHost, cfg.B2NodeGRPCPort), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &NodeClient{
		PrivateKey:      pk,
		ChainID:         cfg.B2NodeChainID,
		CoinDenom:       cfg.B2NodeDenom,
		GrpcHost:        cfg.B2NodeGRPCHost,
		GrpcPort:        cfg.B2NodeGRPCPort,
		AddressPrefix:   prefix,
		B2NodeAddress:   b2NodeAddress,
		UnsignedTxLimit: cfg.B2NodeUnsignedTxLimit,
		GrpcConn:        conn,
	}, nil
}

func (n *NodeClient) GetGrpcConn() *grpc.ClientConn {
	return n.GrpcConn
}

func (n *NodeClient) CloseGrpc() error {
	return n.GrpcConn.Close()
}

func (n *NodeClient) GetAccountInfo(ctx context.Context) (*eTypes.EthAccount, error) {
	conn := n.GetGrpcConn()
	authClient := authTypes.NewQueryClient(conn)
	res, err := authClient.Account(ctx, &authTypes.QueryAccountRequest{Address: n.B2NodeAddress})
	if err != nil {
		return nil, fmt.Errorf("GetAccountInfo err: %w", err)
	}
	ethAccount := &eTypes.EthAccount{}
	err = ethAccount.Unmarshal(res.GetAccount().GetValue())
	if err != nil {
		return nil, err
	}
	return ethAccount, nil
}

func (n *NodeClient) GetGasPrice(ctx context.Context) (uint64, error) {
	baseFee, err := n.BaseFee(ctx)
	if err != nil {
		return 0, err
	}
	return baseFee, nil
}

func (n *NodeClient) broadcastTx(ctx context.Context, msgs ...sdk.Msg) (*tx.BroadcastTxResponse, error) {
	gasPrice, err := n.GetGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetGasPrice err: %w", err)
	}
	txBytes, err := n.buildSimTx(ctx, gasPrice, msgs...)
	if err != nil {
		return nil, fmt.Errorf("buildSimTx err: %w", err)
	}

	txClient := tx.NewServiceClient(n.GetGrpcConn())
	res, err := txClient.BroadcastTx(ctx, &tx.BroadcastTxRequest{
		Mode:    tx.BroadcastMode_BROADCAST_MODE_BLOCK,
		TxBytes: txBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("BroadcastTx err: %w", err)
	}
	return res, err
}

func (n *NodeClient) buildSimTx(ctx context.Context, gasPrice uint64, msgs ...sdk.Msg) ([]byte, error) {
	encCfg := simapp.MakeTestEncodingConfig()
	txBuilder := encCfg.TxConfig.NewTxBuilder()
	err := txBuilder.SetMsgs(msgs...)
	if err != nil {
		return nil, fmt.Errorf("SetMsgs err: %w", err)
	}

	ethAccount, err := n.GetAccountInfo(ctx)
	if nil != err {
		return nil, fmt.Errorf("GetAccountInfo err: %w", err)
	}
	signV2 := signing.SignatureV2{
		PubKey: n.PrivateKey.PubKey(),
		Data: &signing.SingleSignatureData{
			SignMode: encCfg.TxConfig.SignModeHandler().DefaultMode(),
		},
		Sequence: ethAccount.BaseAccount.Sequence,
	}
	err = txBuilder.SetSignatures(signV2)
	if err != nil {
		return nil, fmt.Errorf("SetSignatures err: %w", err)
	}
	txBuilder.SetGasLimit(DefaultBaseGasPrice)
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.Coin{
		Denom:  n.CoinDenom,
		Amount: sdkmath.NewIntFromUint64(gasPrice * DefaultBaseGasPrice),
	}))

	signerData := xauthsigning.SignerData{
		ChainID:       n.ChainID,
		AccountNumber: ethAccount.BaseAccount.AccountNumber,
		Sequence:      ethAccount.BaseAccount.Sequence,
	}

	sigV2, err := clientTx.SignWithPrivKey(
		encCfg.TxConfig.SignModeHandler().DefaultMode(), signerData,
		txBuilder, &n.PrivateKey, encCfg.TxConfig, ethAccount.BaseAccount.Sequence)
	if err != nil {
		return nil, fmt.Errorf("SignWithPrivKey err: %w", err)
	}

	err = txBuilder.SetSignatures(sigV2)
	if err != nil {
		return nil, fmt.Errorf("SetSignatures 2 err: %w", err)
	}
	txBytes, err := encCfg.TxConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return nil, fmt.Errorf("GetTx err: %w", err)
	}
	return txBytes, err
}

func (n *NodeClient) Unsigned(ctx context.Context) ([]bridgeTypes.Withdraw, error) {
	queryClient := bridgeTypes.NewQueryClient(n.GetGrpcConn())
	res, err := queryClient.WithdrawsByStatus(ctx, &bridgeTypes.QueryWithdrawsByStatusRequest{
		Status: bridgeTypes.WithdrawStatus_WITHDRAW_STATUS_PENDING,
		Pagination: &query.PageRequest{
			Limit: n.UnsignedTxLimit,
		},
	})
	if err != nil {
		return nil, err
	}
	return res.Withdraw, nil
}

func (n *NodeClient) Sign(ctx context.Context, hash string, pack *psbt.Packet) error {
	if len(pack.Inputs) == 0 {
		return fmt.Errorf("psbt pack.Inputs is empty")
	}
	var sign []Sign
	for index, input := range pack.Inputs {
		sign = append(sign, Sign{
			TxInIndex: index,
			Sign:      input.FinalScriptSig,
		})
	}

	signJSON, err := json.Marshal(sign)
	if err != nil {
		return err
	}
	msg := bridgeTypes.NewMsgSignWithdraw(n.B2NodeAddress, hash, hex.EncodeToString(signJSON))
	msgResponse, err := n.broadcastTx(ctx, msg)
	if err != nil {
		return fmt.Errorf("broadcastTx err: %w", err)
	}
	code := msgResponse.TxResponse.Code
	rawLog := msgResponse.TxResponse.RawLog
	if code != 0 {
		return fmt.Errorf("code: %d, err: %s", code, rawLog)
	}
	log.Printf("sign success, btc hash:%s, b2node tx hash:%s", hash, msgResponse.TxResponse.TxHash)
	return nil
}

func (n *NodeClient) BaseFee(ctx context.Context) (uint64, error) {
	queryClient := feeTypes.NewQueryClient(n.GetGrpcConn())
	res, err := queryClient.Params(ctx, &feeTypes.QueryParamsRequest{})
	if err != nil {
		return 0, err
	}
	return res.Params.BaseFee.Uint64(), nil
}

func Bech32Prefix(host string, port uint32) (string, error) {
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", host, port), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", err
	}
	defer conn.Close()
	queryClient := authTypes.NewQueryClient(conn)
	bech32Prefix, err := queryClient.Bech32Prefix(context.Background(), &authTypes.Bech32PrefixRequest{})
	if err != nil {
		return "", err
	}
	return bech32Prefix.Bech32Prefix, nil
}

func b2NodeAddress(privateKey ethsecp256k1.PrivKey, prefix string) (string, error) {
	privKey, err := privateKey.ToECDSA()
	if err != nil {
		return "", err
	}
	ethAddress := crypto.PubkeyToAddress(privKey.PublicKey)
	bz, err := hex.DecodeString(ethAddress.Hex()[2:])
	if err != nil {
		return "", err
	}
	b2nodeAddress, err := bech32.ConvertAndEncode(prefix, bz)
	if err != nil {
		return "", err
	}
	return b2nodeAddress, nil
}

func EcdsaToB2NodeAddress(publicKey ecdsa.PublicKey, prefix string) (string, string, error) {
	pubBytes := crypto.FromECDSAPub(&publicKey)
	ethAddress := common.BytesToAddress(crypto.Keccak256(pubBytes[1:])[12:])
	bz, err := hex.DecodeString(ethAddress.Hex()[2:])
	if err != nil {
		return "", "", err
	}
	b2nodeAddress, err := bech32.ConvertAndEncode(prefix, bz)
	if err != nil {
		return "", "", err
	}
	return b2nodeAddress, ethAddress.Hex(), nil
}
