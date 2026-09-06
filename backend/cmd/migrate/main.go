// Comando: aplica/desfaz migrações do banco. Uso: migrate [up|down|status].
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/joho/godotenv"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/config"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/db"
)

func main() {
	_ = godotenv.Load()
	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	cfg, err := config.Load()
	if err != nil {
		fatal(err)
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal(err)
	}
	defer pool.Close()

	switch cmd {
	case "up":
		if err := db.Migrate(ctx, pool); err != nil {
			fatal(err)
		}
		fmt.Println("ok: migrações aplicadas")
	case "down":
		if err := db.MigrateDown(ctx, pool); err != nil {
			fatal(err)
		}
		fmt.Println("ok: última migração desfeita")
	case "status":
		if err := db.MigrateStatus(ctx, pool); err != nil {
			fatal(err)
		}
	default:
		fatal(fmt.Errorf("uso: migrate [up|down|status]"))
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "erro:", err)
	os.Exit(1)
}
