package main

import (
	"log"
	"net/url"

	"github.com/spf13/cobra"

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
	target, err := url.Parse(config.Target)
	if err != nil {
		panic(err)
	}

	ctx := cmd.Context()

	db, err := db.Open(ctx, config.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to cache database: %s", err)
	}

	err = db.MigrateToLatest(ctx)
	if err != nil {
		log.Fatalf("failed to migrate database schema: %s", err)
	}

	return proxy.New(config.Address, target, db).Run(ctx)
}
