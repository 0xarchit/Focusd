package core

import (
	"encoding/base64"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	lastNotification time.Time
	notificationMu   sync.Mutex
)

func showNotification(title, message string) bool {
	notificationMu.Lock()
	if time.Since(lastNotification) < 10*time.Second {
		notificationMu.Unlock()
		return false
	}
	lastNotification = time.Now()
	notificationMu.Unlock()

	go func() {
		t, m := title, message
		if len(t) > 200 {
			t = t[:200]
		}
		if len(m) > 200 {
			m = m[:200]
		}

		xmlStr := `<toast><header id='focusd_group' title='Focusd'/><visual><binding template='ToastGeneric'><text id='1'></text><text id='2'></text></binding></visual></toast>`
		b64XML := base64.StdEncoding.EncodeToString([]byte(xmlStr))

		cmd := exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden", "-Command", `
[void][Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime]
[void][Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime]
$xml = [Windows.Data.Xml.Dom.XmlDocument]::new()
$decoded = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("`+b64XML+`"))
$xml.LoadXml($decoded)
$xml.GetElementsByTagName('text').Item(0).InnerText = $env:TTL
$xml.GetElementsByTagName('text').Item(1).InnerText = $env:MSG
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe').Show($toast)
`)
		cmd.Env = append(os.Environ(), "TTL="+t, "MSG="+m)
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
		cmd.Run()
	}()
	return true
}

func showNotificationWithAction(title, message string, callback func(disable bool)) bool {
	notificationMu.Lock()
	if time.Since(lastNotification) < 10*time.Second {
		notificationMu.Unlock()
		return false
	}
	lastNotification = time.Now()
	notificationMu.Unlock()

	go func() {
		t, m := title, message
		if len(t) > 200 {
			t = t[:200]
		}
		if len(m) > 200 {
			m = m[:200]
		}

		xmlStr := `<toast><header id='focusd_group' title='Focusd'/><visual><binding template='ToastGeneric'><text id='1'></text><text id='2'></text></binding></visual><actions><action content='Disable this reminder' arguments='disable' activationType='background'/><action content='Just close' arguments='close' activationType='background'/></actions></toast>`
		b64XML := base64.StdEncoding.EncodeToString([]byte(xmlStr))

		cmd := exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden", "-Command", `
[void][Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime]
[void][Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime]
$xml = [Windows.Data.Xml.Dom.XmlDocument]::new()
$decoded = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("`+b64XML+`"))
$xml.LoadXml($decoded)
$xml.GetElementsByTagName('text').Item(0).InnerText = $env:TTL
$xml.GetElementsByTagName('text').Item(1).InnerText = $env:MSG
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
$evt = Register-ObjectEvent -InputObject $toast -EventName Activated -SourceIdentifier ToastAct 2>$null
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe').Show($toast)
$r = Wait-Event -SourceIdentifier ToastAct -Timeout 120 2>$null
if ($r -ne $null) { $r.MessageData.Arguments } else { 'timeout' }
Unregister-Event -SourceIdentifier ToastAct -ErrorAction SilentlyContinue
`)
		cmd.Env = append(os.Environ(), "TTL="+t, "MSG="+m)
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}

		out, _ := cmd.Output()
		choice := strings.TrimSpace(string(out))
		if callback != nil {
			if choice == "disable" {
				callback(true)
			}
		}
	}()
	return true
}
