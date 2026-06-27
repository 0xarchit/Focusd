package main

import (
	"focusd/cli"
	"focusd/system"
	"runtime/debug"
)

func main() {
	debug.SetGCPercent(10)

	system.CleanupOldBinary()

	cli.RunDaemon()
}
