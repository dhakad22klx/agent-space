package main

import (
	api "agent-harness/api"
	cli "agent-harness/cli"
	"context"
	"flag"
	"log"
)

func main() {
	httpAddress := flag.String("http", "", "serve the HTTP API on this address instead of starting the CLI")
	flag.Parse()
	if *httpAddress != "" {
		if err := api.ListenAndServe(context.Background(), *httpAddress); err != nil {
			log.Fatal(err)
		}
		return
	}
	cli.StartCli()
}
