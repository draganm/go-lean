package project

import "github.com/urfave/cli/v2"

func Command() *cli.Command {
	return &cli.Command{
		Action: func(ctx *cli.Context) error {
			return nil
		},
	}
}
