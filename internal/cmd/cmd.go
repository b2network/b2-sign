package cmd

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"

	"github.com/b2network/b2-sign/internal/btc"
	"github.com/b2network/b2-sign/internal/server"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func Execute() {
	err := rootCmd().Execute()
	if err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "b2-sign",
		Short: "b2 sign",
		Long:  "b2-sign is a application that signed btc transaction",
	}

	rootCmd.AddCommand(startCmd())
	rootCmd.AddCommand(genMultiScript())

	return rootCmd
}

func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "start sign service",
		Run: func(cmd *cobra.Command, _ []string) {
			var (
				mnemonic     string
				mnemonicPass string
			)
			if term.IsTerminal(int(syscall.Stdin)) {
				fmt.Print("Enter mnemonic: ")
				mnemonicStdin, _ := term.ReadPassword(int(syscall.Stdin))
				fmt.Println()
				mnemonic = string(mnemonicStdin)
				fmt.Print("Enter mnemonic password: ")
				password, _ := term.ReadPassword(int(syscall.Stdin))
				fmt.Println()
				mnemonicPass = string(password)
			} else {
				fmt.Println("Error: Cannot read password from non-terminal input")
				return
			}
			derive, err := cmd.Flags().GetString("derive")
			if err != nil {
				log.Println("read derive err:", err.Error())
				return
			}
			err = server.Start(mnemonic, mnemonicPass, derive)
			if err != nil {
				log.Println("start sign service failed:", err.Error())
				return
			}
		},
	}
	cmd.Flags().StringP("derive", "d", "m/48'/1'/0'/2'/0/1/0/0", "derive path")
	return cmd
}

func genMultiScript() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "multi",
		Short: "gen btc multisig address & script",
		Long:  "gen btc multisig address & script, eg: multi 2 xpub1 xpub2 xpub3",
		Run: func(cmd *cobra.Command, args []string) {
			testnet, err := cmd.Flags().GetBool("testnet")
			if err != nil {
				log.Println(err.Error())
				return
			}
			signNum, err := cmd.Flags().GetInt("signum")
			if err != nil {
				log.Println(err.Error())
				return
			}
			xpubStr, err := cmd.Flags().GetString("xpubs")
			if err != nil {
				log.Println(err.Error())
				return
			}
			xpubs := strings.Split(xpubStr, ",")
			address, script, err := btc.GenerateMultiSigScript(xpubs, signNum, testnet)
			if err != nil {
				log.Println(err.Error())
				return
			}
			log.Println("multisig address: ", address)
			log.Println("multisig script: ", hex.EncodeToString(script))
		},
	}
	cmd.Flags().BoolP("testnet", "t", false, "testnet flag")
	cmd.Flags().IntP("signum", "n", 1, "min sig num")
	cmd.Flags().StringP("xpubs", "x", "", "sign xpub eg: \"xpub1, xpub2, xpub3\"")
	return cmd
}
