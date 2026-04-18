package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/keystore"
	"golang.org/x/term"
)

// confirmWalletNetworkForOperation prompts or errors if the wallet file lists networks
// and the current --network is not among them (unless force is set).
func confirmWalletNetworkForOperation(cfg *config.Config, walletName string, force bool) error {
	recorded, err := keystore.ReadWalletNetworks(cfg.KeystoreDir, walletName)
	if err != nil {
		return err
	}
	if !keystore.NetworkOperationNeedsConfirmation(recorded, string(cfg.Network)) {
		return nil
	}
	fmt.Fprintf(os.Stderr, "Warning: wallet %q was previously used on network(s): %s\n", walletName, strings.Join(recorded, ", "))
	fmt.Fprintf(os.Stderr, "The current operation uses network %q, which is not in that list.\n", cfg.Network)
	fmt.Fprintf(os.Stderr, "This wallet has already been used in operations on the listed network(s).\n")
	if force {
		return nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) { //nolint:gosec // G115
		return fmt.Errorf("network mismatch: use --force to proceed without a terminal, or run interactively to confirm")
	}
	fmt.Fprintf(os.Stderr, "Continue? [y/N]: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return err
	}
	line = strings.TrimSpace(strings.ToLower(line))
	if line == "y" || line == "yes" {
		return nil
	}
	return fmt.Errorf("aborted")
}
