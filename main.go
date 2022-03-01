package main

import (
	"context"
	"net/url"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/kaloyan-raev/ipfs-user-mapping-proxy/proxy"
	"github.com/spf13/cobra"

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

	ctx := context.Background()

	db, err := pgxpool.Connect(ctx, config.DatabaseURL)
	if err != nil {
		panic(err)
	}

	return proxy.New(config.Address, target, db).Run(ctx)
}
