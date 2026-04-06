package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/basant-rai/gomigrate/pkg/migrator"
	_ "github.com/lib/pq"
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	command := os.Args[1]

	switch command {
	case "init":
		runInit()

	case "create":
		if len(os.Args) < 3 {
			fmt.Println("❌ Usage: gomigrate create <name>")
			os.Exit(1)
		}
		runCreate(os.Args[2], flagValue("--dir", "migrations"))

	case "diff":
		db := connectDB()
		defer db.Close()
		m := buildMigrator(db, flagValue("--dir", "migrations"))
		if err := m.Status(); err != nil {
			fmt.Printf("❌ %v\n", err)
			os.Exit(1)
		}

	case "generate":
		if len(os.Args) < 3 {
			fmt.Println("❌ Usage: gomigrate generate <name>")
			os.Exit(1)
		}
		name := os.Args[2]
		db := connectDB()
		defer db.Close()
		dir := flagValue("--dir", "migrations")
		m := buildMigrator(db, dir)

		fmt.Printf("🔍 Scanning models against DB...\n\n")
		m.Status()

		up, down, err := m.Generate(name)
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			os.Exit(1)
		}
		if up == "" {
			fmt.Println("✅ Nothing to generate — DB is already in sync!")
			return
		}
		fmt.Printf("✅ Generated:\n   %s\n   %s\n\nNext: make migrate-up\n", up, down)

	default:
		fmt.Printf("❌ Unknown command: %s\n\n", command)
		printHelp()
		os.Exit(1)
	}
}

func runInit() {
	dir := flagValue("--dir", "migrations")
	os.MkdirAll(dir, 0755)
	fmt.Printf(`✅ gomigrate initialized!

📁 Created: %s/

Next steps:
  1. Register models in cmd/migrate/main.go
  2. gomigrate create initial_schema   (empty files)
  3. gomigrate generate initial_schema (auto from structs)
  4. make migrate-up
`, dir)
}

func runCreate(name, dir string) {
	os.MkdirAll(dir, 0755)
	version := migrator.NextVersion(dir)
	upFile := filepath.Join(dir, fmt.Sprintf("%s_%s.up.sql", version, name))
	downFile := filepath.Join(dir, fmt.Sprintf("%s_%s.down.sql", version, name))

	os.WriteFile(upFile, []byte(fmt.Sprintf("-- Up migration\n\nSELECT '✅ %s_%s applied' AS status;\n", version, name)), 0644)
	os.WriteFile(downFile, []byte(fmt.Sprintf("-- Down migration\n\nSELECT '✅ %s_%s rolled back' AS status;\n", version, name)), 0644)

	fmt.Printf("✅ Created:\n   %s\n   %s\n\nEdit files then: make migrate-up\n", upFile, downFile)
}

func connectDB() *sql.DB {
	dbURL := flagValue("--db", os.Getenv("DATABASE_URL"))
	if dbURL == "" {
		fmt.Println("❌ No DATABASE_URL. Use --db <url> or set DATABASE_URL env var")
		os.Exit(1)
	}
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}
	if err := db.Ping(); err != nil {
		fmt.Printf("❌ DB ping failed: %v\n", err)
		os.Exit(1)
	}
	return db
}

// ⚠️  Register your models here
func buildMigrator(db *sql.DB, dir string) *migrator.Migrator {
	m := migrator.New(db, dir)
	// m.Register(&User{}, "users")
	// m.Register(&CardRelationship{}, "card_relationships")
	// m.Register(&FuelTransaction{}, "fuel_transactions")
	return m
}

func flagValue(flag, defaultVal string) string {
	for i, arg := range os.Args {
		if arg == flag && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
		if strings.HasPrefix(arg, flag+"=") {
			return strings.TrimPrefix(arg, flag+"=")
		}
	}
	return defaultVal
}

func printHelp() {
	fmt.Print(`gomigrate — Auto-generate Postgres migrations from Go structs

COMMANDS:
  init                   Initialize migrations directory
  create  <name>         Create empty migration files
  diff                   Show changes between models and DB
  generate <name>        Auto-generate migration SQL from diff

FLAGS:
  --db <url>             Database URL (or set DATABASE_URL)
  --dir <path>           Migrations directory (default: migrations)

EXAMPLES:
  gomigrate init
  gomigrate create add_users_table
  gomigrate diff
  gomigrate generate add_profile_pic
`)
}
