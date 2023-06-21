package project

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
)

// WARNING: This part is not straight-forward!
// ## context:
// go:embed refuses to embed files that are not part of the module.
// Since the template has it's own `go.mod` file, it is considered to be a separate module.
// ## solution
// There is an open issue about this: https://github.com/golang/go/issues/45197
// Suggested workaround is to create a zip of the template and distribute the 'sub-module' like that.

//go:generate zip -r template.zip template
//go:embed template.zip
var templateZip []byte

func getTemplateFS() (fs.FS, error) {
	return zip.NewReader(bytes.NewReader(templateZip), int64(len(templateZip)))
}

func Command() *cli.Command {
	return &cli.Command{
		Flags: []cli.Flag{},
		Name:  "project",
		Action: func(c *cli.Context) error {
			if c.NArg() != 1 {
				return fmt.Errorf("directory name must be provided")
			}

			dir := c.Args().First()

			templateFS, err := getTemplateFS()
			if err != nil {
				return fmt.Errorf("could not open template fs: %w", err)
			}

			err = fs.WalkDir(templateFS, "template", func(pth string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				withoutPrefix := path.Clean(strings.TrimPrefix(pth, "template"))
				withDirectory := filepath.Join(dir, filepath.FromSlash(withoutPrefix))
				fmt.Println(withDirectory)
				if d.IsDir() {
					err = os.Mkdir(withDirectory, 0770)
					if err != nil {
						return fmt.Errorf("could not mkdir %s: %w", withDirectory, err)
					}
					return nil
				}

				f, err := templateFS.Open(pth)
				if err != nil {
					return fmt.Errorf("could not open template %s: %w", pth, err)
				}

				defer f.Close()
				of, err := os.Create(withDirectory)
				if err != nil {
					return fmt.Errorf("could not create file %s: %w", withDirectory, err)
				}
				defer of.Close()
				_, err = io.Copy(of, f)
				if err != nil {
					return fmt.Errorf("could not copy template %s to file %s: %w", pth, withDirectory, err)
				}
				return err
			})

			if err != nil {
				return err
			}

			return nil
		},
	}
}
