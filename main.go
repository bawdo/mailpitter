package main

import (
	"os"

	"github.com/bawdo/mailpitter/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
