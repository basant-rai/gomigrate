package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"

	// _ "github.com/lib/pq"
	"github.com/basant-rai/gomigrate/pkg/migrator"
)

func main() {
	// Commands
	diffCmd := flag.Bool("diff", false, "Show detected changes between models and DB")
	generateCmd := flag.String("generate", "", "Generate migration files (provide a name)")
	dbURL := flag.String("db", os.Getenv("DATABASE_URL"), "Database URL")
	migrationsDir := flag.String("dir", "migrations", "Migrations directory")

	flag.Parse()

	if *dbURL == "" {
		fmt.Println("❌ No database URL provided. Use --db or set DATABASE_URL")
		os.Exit(1)
	}

	// Connect to DB
	db, err := sql.Open("postgres", *dbURL)
	if err != nil {
		fmt.Printf("❌ DB connection failed: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		fmt.Printf("❌ DB ping failed: %v\n", err)
		os.Exit(1)
	}

	// Build migrator with your models
	// ⚠️  Register your models here
	m := migrator.New(db, *migrationsDir)
	// m.Register(&auth.User{}, "users")
	// m.Register(&card.CardRelationship{}, "card_relationships")
	// m.Register(&fuel.FuelTransaction{}, "fuel_transactions")

	switch {
	case *diffCmd:
		if err := m.Status(); err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			os.Exit(1)
		}

	case *generateCmd != "":
		fmt.Printf("🔍 Scanning models against DB...\n\n")
		if err := m.Status(); err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			os.Exit(1)
		}
		up, down, err := m.Generate(*generateCmd)
		if err != nil {
			fmt.Printf("❌ Generation failed: %v\n", err)
			os.Exit(1)
		}
		if up == "" {
			fmt.Println("✅ Nothing to generate — DB is already in sync!")
			return
		}
		fmt.Printf("✅ Generated migration files:\n")
		fmt.Printf("   %s\n", up)
		fmt.Printf("   %s\n", down)
		fmt.Printf("\nRun: make migrate-up\n")

	default:
		printHelp()
	}
}

func printHelp() {
	fmt.Println(`
gomigrate — Auto-generate migrations from Go structs

USAGE:
  gomigrate [command] [flags]

COMMANDS:
  --diff                    Show detected changes between models and DB
  --generate <name>         Generate migration files from detected changes

FLAGS:
  --db <url>                Database URL (or set DATABASE_URL env var)
  --dir <path>              Migrations directory (default: migrations)

EXAMPLES:
  # Check what changed
  gomigrate --diff --db "postgresql://admin:admin@localhost:5432/wallet"

  # Generate migration
  gomigrate --generate add_profile_pic --db "postgresql://admin:admin@localhost:5432/wallet"

  # Using env var
  export DATABASE_URL="postgresql://admin:admin@localhost:5432/wallet"
  gomigrate --diff
  gomigrate --generate add_profile_pic
`)
}
