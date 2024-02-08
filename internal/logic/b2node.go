package logic

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/b2network/b2-sign/internal/config"
	"github.com/btcsuite/btcd/btcutil/psbt"
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
	PrivateKey    ethsecp256k1.PrivKey
	ChainID       string
	GrpcHost      string
	GrpcPort      uint32
	CoinDenom     string
	GasPrices     uint64
	AddressPrefix string
}

func NewNodeClient(
	cfg *config.Config,
) (*NodeClient, error) {
	privatekeyBytes, err := hex.DecodeString(cfg.B2NodePrivKey)
	if nil != err {
		return nil, err
	}

	prefix, err := B2NodeBech32Prefix(cfg.B2NodeGRPCHost, cfg.B2NodeGRPCPort)
	if err != nil {
		return nil, err
	}
	return &NodeClient{
		PrivateKey: ethsecp256k1.PrivKey{
			Key: privatekeyBytes,
		},
		ChainID:       cfg.B2NodeChainID,
		CoinDenom:     cfg.B2NodeDenom,
		GrpcHost:      cfg.B2NodeGRPCHost,
		GrpcPort:      cfg.B2NodeGRPCPort,
		AddressPrefix: prefix,
	}, nil
}

func (n *NodeClient) grpcConn() (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", n.GrpcHost, n.GrpcPort), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (n *NodeClient) GetAccountInfo() (*eTypes.EthAccount, error) {
	b2nodeAddress, err := n.B2NodeSenderAddress()
	if err != nil {
		return nil, err
	}

	conn, err := n.grpcConn()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	authClient := authTypes.NewQueryClient(conn)
	res, err := authClient.Account(context.Background(), &authTypes.QueryAccountRequest{Address: b2nodeAddress})
	if err != nil {
		return nil, fmt.Errorf("[NodeClient] GetAccountInfo err: %w", err)
	}
	ethAccount := &eTypes.EthAccount{}
	err = ethAccount.Unmarshal(res.GetAccount().GetValue())
	if err != nil {
		return nil, fmt.Errorf("[NodeClient][ethAccount.Unmarshal] err: %w", err)
	}
	return ethAccount, nil
}

func (n *NodeClient) GetGasPrice() (uint64, error) {
	baseFee, err := n.BaseFee()
	if err != nil {
		return 0, err
	}
	baseFee = baseFee * 2
	return baseFee, nil
}

func (n *NodeClient) broadcastTx(ctx context.Context, msgs ...sdk.Msg) (*tx.BroadcastTxResponse, error) {
	gasPrice, err := n.GetGasPrice()
	if err != nil {
		return nil, fmt.Errorf("[broadcastTx][GetEthGasPrice] err: %w", err)
	}
	txBytes, err := n.buildSimTx(gasPrice, msgs...)
	if err != nil {
		return nil, fmt.Errorf("[broadcastTx] err: %w", err)
	}

	conn, err := n.grpcConn()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	txClient := tx.NewServiceClient(conn)
	res, err := txClient.BroadcastTx(ctx, &tx.BroadcastTxRequest{
		Mode:    tx.BroadcastMode_BROADCAST_MODE_BLOCK,
		TxBytes: txBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("[broadcastTx] err: %w", err)
	}
	return res, err
}

func (n *NodeClient) buildSimTx(gasPrice uint64, msgs ...sdk.Msg) ([]byte, error) {
	encCfg := simapp.MakeTestEncodingConfig()
	txBuilder := encCfg.TxConfig.NewTxBuilder()
	err := txBuilder.SetMsgs(msgs...)
	if err != nil {
		return nil, fmt.Errorf("[BuildSimTx][SetMsgs] err: %w", err)
	}

	ethAccount, err := n.GetAccountInfo()
	if nil != err {
		return nil, fmt.Errorf("[BuildSimTx][GetAccountInfo]err: %w", err)
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
		return nil, fmt.Errorf("[BuildSimTx][SetSignatures 1]err: %w", err)
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
		return nil, fmt.Errorf("[BuildSimTx][SignWithPrivKey] err: %w", err)
	}

	err = txBuilder.SetSignatures(sigV2)
	if err != nil {
		return nil, fmt.Errorf("[BuildSimTx][SetSignatures 2] err: %w", err)
	}
	txBytes, err := encCfg.TxConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return nil, fmt.Errorf("[BuildSimTx][GetTx] err: %w", err)
	}
	return txBytes, err
}

func (n *NodeClient) B2NodeSenderAddress() (string, error) {
	privateKey, err := n.PrivateKey.ToECDSA()
	if err != nil {
		return "", err
	}
	ethAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	bz, err := hex.DecodeString(ethAddress.Hex()[2:])
	if err != nil {
		return "", err
	}
	b2nodeAddress, err := bech32.ConvertAndEncode(n.AddressPrefix, bz)
	if err != nil {
		return "", err
	}
	return b2nodeAddress, nil
}

func (n *NodeClient) Unsigned() ([]bridgeTypes.Withdraw, error) {
	conn, err := n.grpcConn()
	if err != nil {
		return nil, err
	}
	queryClient := bridgeTypes.NewQueryClient(conn)
	res, err := queryClient.WithdrawsByStatus(context.Background(), &bridgeTypes.QueryWithdrawsByStatusRequest{
		Status: bridgeTypes.WithdrawStatus_WITHDRAW_STATUS_PENDING,
		Pagination: &query.PageRequest{
			Limit: 100,
		},
	})
	if err != nil {
		return nil, err
	}
	return res.Withdraw, nil
}

func (n *NodeClient) Sign(hash string, pack *psbt.Packet) error {
	senderAddress, err := n.B2NodeSenderAddress()
	if err != nil {
		return err
	}
	if len(pack.Inputs) == 0 {
		return fmt.Errorf("psbt pack.Inputs is empty")
	}

	packB64, err := pack.B64Encode()
	if err != nil {
		return err
	}

	msg := bridgeTypes.NewMsgSignWithdraw(senderAddress, hash, packB64)
	ctx := context.Background()
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

func (n *NodeClient) BaseFee() (uint64, error) {
	conn, err := n.grpcConn()
	if err != nil {
		return 0, err
	}
	queryClient := feeTypes.NewQueryClient(conn)
	res, err := queryClient.Params(context.Background(), &feeTypes.QueryParamsRequest{})
	if err != nil {
		return 0, err
	}
	return res.Params.BaseFee.Uint64(), nil
}

func B2NodeBech32Prefix(host string, port uint32) (string, error) {
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
