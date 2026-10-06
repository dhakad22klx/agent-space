package main

import (
	"context"
	cli "justsay-harness/cli"
	"os"
)

func main() {
	os.Exit(cli.Run(context.Background(), os.Args[1:]))
}
