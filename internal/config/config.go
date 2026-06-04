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

// Package config holds runtime configuration for the monujo CLI (network, endpoints, output).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Network identifies Tron mainnet or Shasta testnet.
type Network string

const (
	// NetworkMainnet is TRON mainnet.
	NetworkMainnet Network = "mainnet"
	// NetworkShasta is the Shasta public testnet.
	NetworkShasta Network = "shasta"
)

// OutputFormat controls how results are printed.
type OutputFormat string

const (
	// OutputTable prints human-readable tables (default).
	OutputTable OutputFormat = "table"
	// OutputJSON prints machine-readable JSON.
	OutputJSON OutputFormat = "json"
)

// WriteTransport selects how monujo talks to a Tron full node for writes.
type WriteTransport string

const (
	// WriteTransportREST uses HTTPS Wallet HTTP API (default, TronGrid-safe).
	WriteTransportREST WriteTransport = "rest"
	// WriteTransportGRPC uses gotron-sdk gRPC (opt-in; often plaintext).
	WriteTransportGRPC WriteTransport = "grpc"
)

// Config aggregates CLI flags and resolved endpoints.
type Config struct {
	Network        Network
	HTTPBaseURL    string
	GRPCAddress    string
	TronGridAPIKey string
	Output         OutputFormat
	KeystoreDir    string
	WriteTransport WriteTransport
	GRPCInsecure   bool
	// DefaultFeeLimitSun is the maximum fee (sun) for TRC-20 contract calls.
	DefaultFeeLimitSun int64
}

const (
	defaultTronGridHTTPMainnet = "https://api.trongrid.io"
	defaultTronGridHTTPShasta  = "https://api.shasta.trongrid.io"
	defaultGRPCMainnet         = "grpc.trongrid.io:50051"
	defaultGRPCShasta          = "grpc.shasta.trongrid.io:50051"
	defaultFeeLimitSun         = int64(100_000_000)
)

// DefaultKeystoreDir returns ~/.config/monujo/keystore unless MONUJO_KEYSTORE_DIR is set.
func DefaultKeystoreDir() (string, error) {
	if v := strings.TrimSpace(os.Getenv("MONUJO_KEYSTORE_DIR")); v != "" {
		return filepath.Abs(v)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "monujo", "keystore"), nil
}

// TronGridAPIKeyFromEnv returns optional TronGrid API key from MONUJO_TRONGRID_API_KEY.
func TronGridAPIKeyFromEnv() string {
	return strings.TrimSpace(os.Getenv("MONUJO_TRONGRID_API_KEY"))
}

// ParseNetwork normalizes the network flag value.
func ParseNetwork(s string) (Network, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "mainnet", "":
		return NetworkMainnet, nil
	case "shasta":
		return NetworkShasta, nil
	default:
		return "", fmt.Errorf("unknown network %q (use mainnet or shasta)", s)
	}
}

// ParseWriteTransport parses --write-transport.
func ParseWriteTransport(s string) (WriteTransport, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "rest", "":
		return WriteTransportREST, nil
	case "grpc":
		return WriteTransportGRPC, nil
	default:
		return "", fmt.Errorf("unknown write transport %q (use rest or grpc)", s)
	}
}

// IsTronGridGRPC reports whether addr is a known public TronGrid gRPC endpoint.
func IsTronGridGRPC(addr string) bool {
	switch strings.TrimSpace(addr) {
	case defaultGRPCMainnet, defaultGRPCShasta:
		return true
	default:
		return false
	}
}

// ParseOutputFormat parses --output.
func ParseOutputFormat(s string) (OutputFormat, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "table", "":
		return OutputTable, nil
	case "json":
		return OutputJSON, nil
	default:
		return "", fmt.Errorf("unknown output format %q (use table or json)", s)
	}
}

// Build constructs a Config from parts.
func Build(network Network, output OutputFormat, httpOverride, grpcOverride, apiKey string, keystoreDir string) (*Config, error) {
	var httpURL, grpcAddr string
	switch network {
	case NetworkMainnet:
		httpURL = defaultTronGridHTTPMainnet
		grpcAddr = defaultGRPCMainnet
	case NetworkShasta:
		httpURL = defaultTronGridHTTPShasta
		grpcAddr = defaultGRPCShasta
	default:
		return nil, fmt.Errorf("unsupported network %q", network)
	}
	if strings.TrimSpace(httpOverride) != "" {
		httpURL = strings.TrimSpace(httpOverride)
	}
	if strings.TrimSpace(grpcOverride) != "" {
		grpcAddr = strings.TrimSpace(grpcOverride)
	}
	if keystoreDir == "" {
		var err error
		keystoreDir, err = DefaultKeystoreDir()
		if err != nil {
			return nil, err
		}
	}
	return &Config{
		Network:            network,
		HTTPBaseURL:        strings.TrimRight(httpURL, "/"),
		GRPCAddress:        grpcAddr,
		TronGridAPIKey:     strings.TrimSpace(apiKey),
		Output:             output,
		KeystoreDir:        keystoreDir,
		WriteTransport:     WriteTransportREST,
		GRPCInsecure:       false,
		DefaultFeeLimitSun: defaultFeeLimitSun,
	}, nil
}
