package main

import (
	"github.com/draganm/go-lean/cmd/generate"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Commands: []*cli.Command{
			generate.Command(),
		},
	}
	app.RunAndExitOnError()
}
