package main

import (
	"focusd/cli"
	"focusd/system"
	"os"
)

func main() {
	system.CleanupOldBinary()

	cli.Run(os.Args)
}
