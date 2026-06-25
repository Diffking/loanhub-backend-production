package main

// Standalone CLI tool — seed loan_purposes from FLOPRESN.txt.
//
// Usage:
//   go run cmd/seed-loan-purposes/main.go [path/to/FLOPRESN.txt]
//
// Default path: data/FLOPRESN.txt (relative to CWD)
//
// Run once after deploying Phase 1 backend; future updates can be triggered
// by re-running this command (idempotent — inserts new codes, updates existing names).

import (
	"log"
	"os"

	"spsc-loaneasy/internal/config"
)

func main() {
	path := "data/FLOPRESN.txt"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := config.ConnectDatabase(cfg)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}

	if err := config.SeedLoanPurposes(db, path); err != nil {
		log.Fatalf("seed loan purposes: %v", err)
	}

	log.Println("Done.")
}
