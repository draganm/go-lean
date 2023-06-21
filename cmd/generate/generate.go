package generate

import (
	"github.com/draganm/go-lean/cmd/generate/project"
	"github.com/urfave/cli/v2"
)

func Command() *cli.Command {
	return &cli.Command{
		Name:        "generate",
		Subcommands: []*cli.Command{project.Command()},
	}
}
