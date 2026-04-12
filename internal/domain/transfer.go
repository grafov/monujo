package domain

import "time"

// TRC20TransferRow is one TRC-20 transfer line from indexer history (watch-only).
type TRC20TransferRow struct {
	TxID           string
	BlockTimestamp time.Time
	From           string
	To             string
	ValueMinimal   string
	TokenSymbol    string
	Contract       string
	Type           string
}
