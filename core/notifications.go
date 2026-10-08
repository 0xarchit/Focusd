package core

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
)

var (
	lastNotification time.Time
	notificationMu   sync.Mutex
)

// toastAUMID identifies focusd itself in the Action Center. Using the
// PowerShell GUID made notifications appear as "Windows PowerShell", so a user
// who had disabled PowerShell notifications never saw focusd alerts, and the
// toast carried no application identity.
const toastAUMID = "Focusd.Desktop"

// notificationProcessWindowHidden avoids flashing a console window.
const notificationProcessWindowHidden = 0x08000000

// notificationMinInterval throttles bursts of toasts.
const notificationMinInterval = 10 * time.Second

// truncateRunes clips a string to at most n runes.
//
// The previous byte slice could split a multi-byte rune in the middle, which
// produced malformed XML and a toast that silently failed to render.
func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}

	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}

// xmlEscape escapes the characters that cannot appear literally in the toast
// XML. The title and message travel through environment variables rather than
// being interpolated, but escaping keeps the document well formed regardless.
func xmlEscape(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(s)
}

func notificationRateLimited() bool {
	notificationMu.Lock()
	defer notificationMu.Unlock()
	if time.Since(lastNotification) < notificationMinInterval {
		return false
	}
	lastNotification = time.Now()
	return true
}

// resetNotificationState clears the throttle. Used by tests.
func resetNotificationState() {
	notificationMu.Lock()
	lastNotification = time.Time{}
	notificationMu.Unlock()
}

// runToast executes the PowerShell toast script, cancelling it when ctx ends.
//
// Every call previously spawned a powershell.exe that lived for up to 120
// seconds waiting on an event, so shutting down left processes behind.
func runToast(ctx context.Context, script string, env []string) []byte {
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive",
		"-WindowStyle", "Hidden", "-Command", script)
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: notificationProcessWindowHidden}

	// Bound the wait so a wedged PowerShell cannot outlive the daemon.
	done := make(chan struct{})
	var out []byte
	go func() {
		out, _ = cmd.Output()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		<-done
	}
	return out
}

const plainToastScript = `
[void][Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime]
[void][Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime]
$xml = [Windows.Data.Xml.Dom.XmlDocument]::new()
$xml.LoadXml($env:FOCUS_TOAST_XML)
$xml.GetElementsByTagName('text').Item(0).InnerText = $env:FOCUS_TITLE
$xml.GetElementsByTagName('text').Item(1).InnerText = $env:FOCUS_MSG
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('` + toastAUMID + `').Show($toast)
`

const actionToastScript = plainToastScript + `
$evt = Register-ObjectEvent -InputObject $toast -EventName Activated -SourceIdentifier FocusdAct 2>$null
$r = Wait-Event -SourceIdentifier FocusdAct -Timeout 120 2>$null
if ($r -ne $null) { $r.MessageData.Arguments } else { 'timeout' }
Unregister-Event -SourceIdentifier FocusdAct -ErrorAction SilentlyContinue
`

const toastXMLTemplate = `<toast><visual><binding template="ToastGeneric"><text id="1"></text><text id="2"></text></binding></visual></toast>`

const actionToastXMLTemplate = `<toast><visual><binding template="ToastGeneric"><text id="1"></text><text id="2"></text></binding></visual><actions><action content="Disable this reminder" arguments="disable" activationType="background"/><action content="Just close" arguments="close" activationType="background"/></actions></toast>`

func toastEnv(title, message, xml string) []string {
	return append(os.Environ(),
		"FOCUS_TITLE="+xmlEscape(title),
		"FOCUS_MSG="+xmlEscape(message),
		"FOCUS_TOAST_XML="+xmlEscape(xml),
	)
}

func showNotification(ctx context.Context, title, message string) bool {
	if !notificationRateLimited() {
		return false
	}

	t := truncateRunes(title, 200)
	m := truncateRunes(message, 200)

	go runToast(ctx, plainToastScript, toastEnv(t, m, toastXMLTemplate))
	return true
}

func showNotificationWithAction(ctx context.Context, title, message string, callback func(disable bool)) bool {
	if !notificationRateLimited() {
		return false
	}

	t := truncateRunes(title, 200)
	m := truncateRunes(message, 200)

	go func() {
		out := runToast(ctx, actionToastScript, toastEnv(t, m, actionToastXMLTemplate))
		choice := strings.TrimSpace(string(out))
		if choice == "disable" && callback != nil {
			callback(true)
		}
	}()
	return true
}

// notifyContext is cancelled when the tracker stops, so in-flight toast
// processes are torn down with it.
var notifyContext, cancelNotify = context.WithCancel(context.Background())

// StopNotifications cancels any in-flight toast process.
func StopNotifications() {
	cancelNotify()
}
