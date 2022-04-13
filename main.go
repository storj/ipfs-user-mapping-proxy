package main

import (
	"fmt"
	"log"
	"net/url"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/ipfs-user-mapping-proxy/db"
	"storj.io/ipfs-user-mapping-proxy/proxy"
	"storj.io/private/process"
)

var (
	rootCmd = &cobra.Command{
		Use:   "ipfs-user-mapping-proxy",
		Short: "IPFS reverse proxy mapping users to content",
	}

	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run the proxy",
		RunE:  cmdRun,
	}

	config struct {
		Address     string `help:"address to listen for incoming requests"`
		Target      string `help:"target url of the IPFS HTTP API to redirect the incoming requests"`
		DatabaseURL string `help:"database url to store user to content mappings"`
	}
)

func init() {
	rootCmd.AddCommand(runCmd)
	process.Bind(runCmd, &config)
}

func main() {
	process.Exec(rootCmd)
}

func cmdRun(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	logger, _, err := process.NewLogger(rootCmd.Use)
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
		return fmt.Errorf("failed to initialize logger: %v", err)
	}

	target, err := url.Parse(config.Target)
	if err != nil {
		logger.Fatal("Failed to parse target url", zap.Error(err))
		return fmt.Errorf("failed to parse target url: %v", err)
	}

	db, err := db.Open(ctx, config.DatabaseURL)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
		return fmt.Errorf("failed to connect to database: %v", err)
	}

	db = db.WithLog(logger)

	err = db.MigrateToLatest(ctx)
	if err != nil {
		logger.Fatal("Failed to migrate database schema", zap.Error(err))
		return fmt.Errorf("failed to migrate database schema: %v", err)
	}

	err = proxy.New(logger, db, config.Address, target).Run(ctx)
	if err != nil {
		logger.Error("Error running proxy", zap.Error(err))
	}

	return err
}
