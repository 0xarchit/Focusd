package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	lastNotificationTime time.Time
	notificationMutex    sync.Mutex
	notificationCooldown = 30 * time.Second
)

func ShowNotification(title, message string) {
	notificationMutex.Lock()
	if time.Since(lastNotificationTime) < notificationCooldown {
		notificationMutex.Unlock()
		return
	}
	lastNotificationTime = time.Now()
	notificationMutex.Unlock()

	title = strings.ReplaceAll(title, "'", "''")
	message = strings.ReplaceAll(message, "'", "''")

	ps := `
$app = '{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe'
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null

$template = @"
<toast>
    <visual>
        <binding template="ToastText02">
            <text id="1">` + title + `</text>
            <text id="2">` + message + `</text>
        </binding>
    </visual>
    <audio src="ms-winsoundevent:Notification.Default"/>
</toast>
"@

$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml($template)
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($app).Show($toast)
`

	appData := os.Getenv("APPDATA")
	if appData == "" {
		return
	}

	psPath := filepath.Join(appData, "focusd", "notify.ps1")
	os.WriteFile(psPath, []byte(ps), 0644)

	exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden", "-ExecutionPolicy", "Bypass", "-File", psPath).Start()
}
