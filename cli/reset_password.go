package cli

import (
	"bufio"
	"fmt"
	"focusd/system"
	"focusd/ui"
	"os"
	"strings"
)

func RunResetPassword() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║                    Password Reset                        ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()

	if !system.IsPasswordEnabled() {
		ui.PrintInfo("No password is currently set.")
		return
	}

	fmt.Println("  [!] Warning: This will remove your password protection.")
	fmt.Println()
	fmt.Print("Type 'yes' to confirm password reset: ")
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(confirm)

	if strings.ToLower(confirm) != "yes" {
		ui.PrintInfo("Cancelled")
		return
	}

	if err := system.ClearPassword(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to reset password: %v", err))
		return
	}

	ui.PrintOK("Password has been reset. Menu is now unlocked.")
}
