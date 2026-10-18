package core

import (
	"bytes"
	"context"
	"focusd/system"
	"log"
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

// toastAUMID identifies focusd in the Action Center rather than borrowing the
// PowerShell GUID, which made alerts appear under the wrong application.
const toastAUMID = "Focusd.Desktop"

// toastDisplayName is the app name Windows shows for focusd's notifications.
const toastDisplayName = "Focusd"

// notificationProcessWindowHidden avoids flashing a console window.
const notificationProcessWindowHidden = 0x08000000

// notificationMinInterval throttles bursts of toasts.
const notificationMinInterval = 10 * time.Second

// truncateRunes clips a string to at most n runes. Byte slicing would split a
// multi-byte rune and produce malformed XML.
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

// ensureToastAppID registers the AUMID before a toast is sent. Windows drops
// toasts whose AUMID is neither registered under Software\Classes\
// AppUserModelId nor backed by a Start Menu shortcut: Show() succeeds and the
// platform even logs delivery, yet nothing is ever displayed.
func ensureToastAppID() {
	iconPath, err := os.Executable()
	if err != nil {
		iconPath = ""
	}
	if err := system.EnsureToastAppID(toastAUMID, toastDisplayName, iconPath); err != nil {
		log.Printf("WARN: failed to register toast app id: %v", err)
	}
}

// toastProcess is a started PowerShell toast script. The process is started
// synchronously so it survives short-lived callers: focusd status can complete
// a pomodoro and exit immediately, and the toast must still appear.
type toastProcess struct {
	stdout *bytes.Buffer
	stderr *strings.Builder
	done   chan struct{}
	runErr error
}

// startToast launches the toast script. Cancelling ctx kills the process, so a
// pending action toast cannot outlive shutdown by its full timeout.
func startToast(ctx context.Context, script string, env []string) *toastProcess {
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive",
		"-WindowStyle", "Hidden", "-Command", script)
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: notificationProcessWindowHidden}

	tp := &toastProcess{
		stdout: &bytes.Buffer{},
		stderr: &strings.Builder{},
		done:   make(chan struct{}),
	}
	cmd.Stdout = tp.stdout
	cmd.Stderr = tp.stderr

	if err := cmd.Start(); err != nil {
		if ctx.Err() == nil {
			log.Printf("WARN: toast script failed to start: %v", err)
		}
		close(tp.done)
		return tp
	}
	go func() {
		tp.runErr = cmd.Wait()
		close(tp.done)
	}()
	return tp
}

// wait blocks until the script exits and returns its stdout.
func (tp *toastProcess) wait(ctx context.Context) []byte {
	<-tp.done

	// A kill during shutdown is expected; any other failure means the toast
	// never appeared and must not stay silent.
	if tp.runErr != nil && ctx.Err() == nil {
		log.Printf("WARN: toast script failed: %v: %s", tp.runErr, strings.TrimSpace(tp.stderr.String()))
	}
	return tp.stdout.Bytes()
}

const plainToastScript = `
$ErrorActionPreference = 'Stop'
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
if ($r -ne $null) { $r.SourceEventArgs.Arguments } else { 'timeout' }
Unregister-Event -SourceIdentifier FocusdAct -ErrorAction SilentlyContinue
`

const toastXMLTemplate = `<toast><visual><binding template="ToastGeneric"><text id="1"></text><text id="2"></text></binding></visual></toast>`

const actionToastXMLTemplate = `<toast><visual><binding template="ToastGeneric"><text id="1"></text><text id="2"></text></binding></visual><actions><action content="Disable this reminder" arguments="disable" activationType="background"/><action content="Just close" arguments="close" activationType="background"/></actions></toast>`

// toastEnv passes values raw. The script assigns title and message through
// InnerText, which escapes on its own, and LoadXml requires the template to be
// literal XML; pre-escaping either made every toast fail to parse.
func toastEnv(title, message, xml string) []string {
	return append(os.Environ(),
		"FOCUS_TITLE="+title,
		"FOCUS_MSG="+message,
		"FOCUS_TOAST_XML="+xml,
	)
}

func showNotification(ctx context.Context, title, message string) bool {
	if !notificationRateLimited() {
		return false
	}
	ensureToastAppID()

	t := truncateRunes(title, 200)
	m := truncateRunes(message, 200)

	tp := startToast(ctx, plainToastScript, toastEnv(t, m, toastXMLTemplate))
	go tp.wait(ctx)
	return true
}

func showNotificationWithAction(ctx context.Context, title, message string, callback func(disable bool)) bool {
	if !notificationRateLimited() {
		return false
	}
	ensureToastAppID()

	t := truncateRunes(title, 200)
	m := truncateRunes(message, 200)

	tp := startToast(ctx, actionToastScript, toastEnv(t, m, actionToastXMLTemplate))
	go func() {
		choice := strings.TrimSpace(string(tp.wait(ctx)))
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
