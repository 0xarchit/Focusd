package main

import (
	"focusd/core"
	"focusd/storage"
	"focusd/system"
	"log"
	"os"
	"time"
)

func main() {
	profileLock, err := system.AcquireProfileLock()
	if err != nil {
		log.Printf("ERROR: daemon profile is already owned: %v", err)
		return
	}
	defer profileLock.Close()
	var dbErr error
	for i := 0; i < 5; i++ {
		dbErr = storage.Init()
		if dbErr == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if dbErr != nil {
		os.Exit(1)
	}
	defer storage.Close()

	tracker := core.NewTracker()
	if err := tracker.Start(); err != nil {
		log.Printf("ERROR: daemon failed to start tracker: %v", err)
		os.Exit(1)
	}
}
