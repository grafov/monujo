// Copyright (C) 2026 Alexander Grafov <grafov@inet.name>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Package cli implements the monujo command-line interface.
package cli

import (
	"fmt"
	"os"

	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/tron"
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
	networkFlag        string
	outputFlag         string
	httpURLFlag        string
	grpcURLFlag        string
	grpcInsecureFlag   bool
	writeTransportFlag string
	apiKeyFlag         string
	keystoreDirFlag    string
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
	root.PersistentFlags().StringVar(&httpURLFlag, "http-url", "", "Override TronGrid HTTP base URL (read + default write transport)")
	root.PersistentFlags().StringVar(&writeTransportFlag, "write-transport", "rest", "Write transport: rest (HTTPS, default) or grpc (opt-in)")
	root.PersistentFlags().StringVar(&grpcURLFlag, "grpc-url", "", "Override Tron gRPC address (host:port); used only with --write-transport grpc")
	root.PersistentFlags().BoolVar(&grpcInsecureFlag, "grpc-insecure", false, "Use plaintext gRPC (with --write-transport grpc; required for TronGrid and most full nodes)")
	root.PersistentFlags().StringVar(&apiKeyFlag, "api-key", "", "TronGrid API key (or env MONUJO_TRONGRID_API_KEY)")
	root.PersistentFlags().StringVar(&keystoreDirFlag, "keystore-dir", "", "Keystore directory (default: ~/.config/monujo/keystore)")

	root.AddCommand(newWatchCmd())
	root.AddCommand(newWalletCmd())
	root.AddCommand(newHotCmd())
	root.AddCommand(newOfflineCmd())
	root.AddCommand(newMultisigCmd())
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
	wt, err := config.ParseWriteTransport(writeTransportFlag)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Build(net, out, httpURLFlag, grpcURLFlag, key, keystoreDirFlag)
	if err != nil {
		return nil, err
	}
	cfg.WriteTransport = wt
	cfg.GRPCInsecure = grpcInsecureFlag
	tron.WarnTronGridPlaintextGRPC(os.Stderr, cfg)
	return cfg, nil
}
