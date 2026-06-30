// Command demografix is the command-line client for the Demografix APIs
// (genderize.io, agify.io, nationalize.io).
package main

import (
	"os"

	"github.com/DemografixGenderize/demografix-cli/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
