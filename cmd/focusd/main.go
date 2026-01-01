package main

import (
	"focusd/cli"
	"focusd/system"
	"os"
	"runtime/debug"
)

func main() {
	debug.SetGCPercent(10)

	system.CleanupOldBinary()

	cli.Run(os.Args)
}
