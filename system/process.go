package system

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func IsProcessRunning(name string) bool {
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq "+name, "/NH")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), name)
}

func KillProcess(name string) error {
	cmd := exec.Command("taskkill", "/F", "/IM", name)
	return cmd.Run()
}

func KillOtherInstances(name string) error {
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq "+name, "/FO", "CSV", "/NH")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	myPID := getMyPID()
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		if strings.Contains(line, name) {
			parts := strings.Split(line, ",")
			if len(parts) >= 2 {
				pidStr := strings.Trim(parts[1], "\" \r")
				pid, err := strconv.Atoi(pidStr)
				if err == nil && pid != myPID {

					exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid)).Run()
				}
			}
		}
	}
	return nil
}

func getMyPID() int {
	return os.Getpid()
}

func GetProcessCount(name string) int {
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq "+name, "/NH")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}
	lines := strings.Split(string(output), "\n")
	count := 0
	for _, line := range lines {
		if strings.Contains(line, name) {
			count++
		}
	}
	return count
}

func GetPIDByName(name string) (uint32, error) {
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq "+name, "/FO", "CSV", "/NH")
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, name) {
			parts := strings.Split(line, ",")
			if len(parts) >= 2 {
				pidStr := strings.Trim(parts[1], "\" \r")
				pid, err := strconv.ParseUint(pidStr, 10, 32)
				if err == nil {
					return uint32(pid), nil
				}
			}
		}
	}
	return 0, nil
}
