package main

import (
	"focusd/cli"
	"runtime/debug"
)

func main() {
	debug.SetGCPercent(10)

	cli.RunDaemon()
}
