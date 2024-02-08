package server

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/b2network/b2-sign/internal/config"
	"github.com/b2network/b2-sign/internal/crypto/bip32"
	"github.com/b2network/b2-sign/internal/logic"
	"github.com/tyler-smith/go-bip39"
)

func Start(mnemonic, mnemonicPass string, derive string) (err error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	if !bip39.IsMnemonicValid(mnemonic) {
		return fmt.Errorf("valid mnemonic")
	}
	seed := bip39.NewSeed(mnemonic, mnemonicPass)
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return err
	}
	childKey, err := masterKey.NewChildKeyByPathString(derive)
	if err != nil {
		return err
	}
	log.Println("xpub: ", childKey.PublicKey())
	log.Println("derive Path: ", derive)
	signKey, err := childKey.ECPrivKey()
	if err != nil {
		log.Println(err)
		return err
	}
	b2NodeClient, err := logic.NewNodeClient(cfg)
	if err != nil {
		log.Println(err)
		return err
	}
	s := logic.NewSignService(cfg, signKey, b2NodeClient)
	errCh := make(chan error)
	go func() {
		if err := s.Start(); err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-time.After(5 * time.Second):
	}
	// wait quit
	code := WaitForQuitSignals()
	log.Println("server exit code:", code)
	return nil
}

func WaitForQuitSignals() int {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT, syscall.SIGHUP)
	sig := <-sigs
	return int(sig.(syscall.Signal)) + 128
}
