package main

import (
	"focusd/cli"
	"focusd/system"
	"os"
	"runtime/debug"
)

func main() {
	system.AttachParentConsole()

	debug.SetGCPercent(10)

	cli.Run(os.Args)
}
