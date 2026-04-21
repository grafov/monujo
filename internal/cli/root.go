// Package cli implements the monujo command-line interface.
package cli

import (
	"fmt"

	"github.com/grafov/monujo/internal/config"
	"github.com/spf13/cobra"
)

var (
	buildVersion = "dev"
	buildCommit  = "unknown"
)

// SetBuildInfo sets version strings injected from main.
func SetBuildInfo(version, commit string) {
	buildVersion = version
	buildCommit = commit
}

var (
	networkFlag     string
	outputFlag      string
	httpURLFlag     string
	grpcURLFlag     string
	apiKeyFlag      string
	keystoreDirFlag string
)

// Execute runs the root command.
func Execute() error {
	root := newRootCmd()
	return root.Execute()
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "monujo",
		Short:         "Minimal Tron (TRC-20) USDT wallet CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       fmt.Sprintf("%s (%s)", buildVersion, buildCommit),
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.PersistentFlags().StringVar(&networkFlag, "network", "mainnet", "Tron network: mainnet or shasta")
	root.PersistentFlags().StringVar(&outputFlag, "output", "table", "Output format: table or json")
	root.PersistentFlags().StringVar(&httpURLFlag, "http-url", "", "Override TronGrid HTTP base URL")
	root.PersistentFlags().StringVar(&grpcURLFlag, "grpc-url", "", "Override Tron gRPC address (host:port)")
	root.PersistentFlags().StringVar(&apiKeyFlag, "api-key", "", "TronGrid API key (or env MONUJO_TRONGRID_API_KEY)")
	root.PersistentFlags().StringVar(&keystoreDirFlag, "keystore-dir", "", "Keystore directory (default: ~/.config/monujo/keystore)")

	root.AddCommand(newWatchCmd())
	root.AddCommand(newWalletCmd())
	root.AddCommand(newHotCmd())
	root.AddCommand(newOfflineCmd())
	root.AddCommand(newToolCmd())
	return root
}

func loadConfig(_ *cobra.Command) (*config.Config, error) {
	net, err := config.ParseNetwork(networkFlag)
	if err != nil {
		return nil, err
	}
	out, err := config.ParseOutputFormat(outputFlag)
	if err != nil {
		return nil, err
	}
	key := apiKeyFlag
	if key == "" {
		key = config.TronGridAPIKeyFromEnv()
	}
	return config.Build(net, out, httpURLFlag, grpcURLFlag, key, keystoreDirFlag)
}
