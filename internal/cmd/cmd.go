package cmd

import (
	"fmt"
	"os"
	"syscall"

	"log"

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

	return rootCmd
}

func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "start sign service",
		Run: func(_ *cobra.Command, _ []string) {
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
			err := server.Start(mnemonic, mnemonicPass)
			if err != nil {
				log.Println("start sign service failed:", err.Error())
			}
		},
	}

	return cmd
}
