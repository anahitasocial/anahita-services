package repositories

import (
	"database/sql"

	"anahita-libs/federation"
)

// The servers that are refused are one list for every protocol, and are
// kept where each service that goes out to other servers can read them:
// anahita-libs/federation.

type (
	DomainBlock     = federation.DomainBlock
	BlocksInterface = federation.BlocksInterface
	Blocks          = federation.Blocks
)

func NewBlocks(db *sql.DB) *Blocks { return federation.NewBlocks(db) }

// CleanDomain is a server's name as it is kept and compared.
func CleanDomain(domain string) string { return federation.CleanDomain(domain) }
