package main

import (
	"focusd/cli"
	"focusd/system"
	"os"
)

func main() {
	system.AttachParentConsole()

	cli.Run(os.Args)
}
