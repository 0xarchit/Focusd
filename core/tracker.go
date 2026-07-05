package core

import (
	"context"
	"fmt"
	"focusd/storage"
	"focusd/system"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const IPCAddress = "127.0.0.1:48321"

func SendIPCCmd(cmd string) bool {
	conn, err := net.DialTimeout("tcp", IPCAddress, 1*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(1 * time.Second)); err != nil {
		return false
	}

	_, err = conn.Write([]byte(cmd))
	if err != nil {
		return false
	}

	buf := make([]byte, 16)
	n, err := conn.Read(buf)
	return err == nil && string(buf[:n]) == "ok"
}

type ActiveSession struct {
	AppName     string
	ExeName     string
	WindowTitle string
	StartTime   time.Time
	Date        string
}

type Tracker struct {
	mu              sync.Mutex
	currentSession  *ActiveSession
	pollInterval    time.Duration
	pendingSessions []*storage.Session
	ctx             context.Context
	cancel          context.CancelFunc
}

func NewTracker() *Tracker {
	ctx, cancel := context.WithCancel(context.Background())
	pollSeconds := storage.GetTrackingIntervalSeconds()
	return &Tracker{
		pollInterval:  time.Duration(pollSeconds) * time.Second,
		ctx:           ctx,
		cancel:        cancel,
	}
}

func (t *Tracker) Start() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		t.Stop()
	}()

	t.recoverOrphanedSession()

	go t.startIPCOnport()

	pollTicker := time.NewTicker(t.pollInterval)
	persistTicker := time.NewTicker(5 * time.Minute)
	retentionTicker := time.NewTicker(1 * time.Hour)
	focusTicker := time.NewTicker(5 * time.Second)
	defer pollTicker.Stop()
	defer persistTicker.Stop()
	defer retentionTicker.Stop()
	defer focusTicker.Stop()

	storage.EnforceRetention()

	var stateMu sync.Mutex
	var continuousUseStart time.Time
	var breakSnoozedUntil time.Time
	prevSessionApp := ""
	disabledLimitApps := make(map[string]time.Time)
	appLimitDate := ""

	for {
		select {
		case <-t.ctx.Done():
			t.flushCurrentSession()
			t.flushPendingSessions()
			storage.ClearActiveSession()
			return
		case <-pollTicker.C:
			if storage.IsPaused() {
				continuousUseStart = time.Time{}
				continue
			}
			t.poll()
			if continuousUseStart.IsZero() {
				continuousUseStart = time.Now()
			}
		case <-persistTicker.C:
			t.flushPendingSessions()
			t.persistActiveSession()
		case <-retentionTicker.C:
			storage.EnforceRetention()
		case <-focusTicker.C:
			CheckPomodoroAndNotify()

			today := storage.Today()
			now := time.Now()
			snoozeDuration := time.Duration(system.GetSnoozeDurationMinutes()) * time.Minute

			stateMu.Lock()
			breakSnoozed := !breakSnoozedUntil.IsZero() && now.Before(breakSnoozedUntil)
			stateMu.Unlock()

			if system.GetBreakReminderEnabled() && !breakSnoozed && !continuousUseStart.IsZero() {
				mins := system.GetBreakReminderMinutes()
				if time.Since(continuousUseStart) >= time.Duration(mins)*time.Minute {
					ShowNotificationWithAction("Break Reminder",
						fmt.Sprintf("You've been working for %d min. Take a break!", mins),
						func(disable bool) {
							stateMu.Lock()
							continuousUseStart = time.Now()
							if disable {
								breakSnoozedUntil = time.Now().Add(snoozeDuration)
							}
							stateMu.Unlock()
						})
				}
			}

			stateMu.Lock()
			if appLimitDate != today {
				disabledLimitApps = make(map[string]time.Time)
				appLimitDate = today
			}
			stateMu.Unlock()

			limits := system.GetAppTimeLimits()

			t.mu.Lock()
			var sessionExe, sessionAppName string
			if t.currentSession != nil {
				sessionExe = strings.ToLower(t.currentSession.ExeName)
				sessionAppName = t.currentSession.AppName
			}
			t.mu.Unlock()

			if len(limits) > 0 && sessionExe != "" {
				if sessionExe != prevSessionApp {
					prevSessionApp = sessionExe

					stateMu.Lock()
					appSnoozed := !disabledLimitApps[sessionExe].IsZero() && now.Before(disabledLimitApps[sessionExe])
					stateMu.Unlock()

					if limit, ok := limits[sessionExe]; ok && !appSnoozed {
						todayUsage := storage.GetAppUsageTodayMinutes(sessionExe)
						if todayUsage >= limit {
							exeCopy := sessionExe
							ShowNotificationWithAction("App Time Limit",
								sessionAppName+" has exceeded daily limit!",
								func(disable bool) {
									if disable {
										stateMu.Lock()
										disabledLimitApps[exeCopy] = time.Now().Add(snoozeDuration)
										stateMu.Unlock()
									}
								})
						}
					}
				}
			}
		}
	}
}

func (t *Tracker) Stop() {
	t.cancel()
}

func (t *Tracker) recoverOrphanedSession() {
	recovered, err := storage.RecoverActiveSession()
	if err != nil || recovered == nil {
		return
	}
	if err := storage.InsertSession(recovered); err != nil {
		log.Printf("ERROR: failed to recover orphaned session (insert): %v", err)
	}
	if err := storage.UpdateAppDaily(recovered.Date, recovered.AppName, recovered.ExeName, recovered.DurationSecs); err != nil {
		log.Printf("ERROR: failed to recover orphaned session (daily): %v", err)
	}
}

func (t *Tracker) persistActiveSession() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.currentSession == nil {
		storage.ClearActiveSession()
		return
	}

	record := &storage.ActiveSessionRecord{
		AppName:     t.currentSession.AppName,
		ExeName:     t.currentSession.ExeName,
		WindowTitle: t.currentSession.WindowTitle,
		StartTime:   t.currentSession.StartTime,
		LastSeen:    time.Now(),
		Date:        t.currentSession.Date,
	}
	storage.SaveActiveSession(record)
}

func (t *Tracker) poll() {
	info, err := system.GetForegroundWindowInfo()
	if err != nil || info == nil || info.Title == "" || info.ExeName == "" {
		return
	}

	if system.IsWhitelisted(info.ExeName) {
		return
	}

	appName := getAppName(info.ExeName)

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.currentSession != nil {
		if t.currentSession.ExeName == info.ExeName {
			return
		}
		t.closeCurrentSession()
	}

	t.currentSession = &ActiveSession{
		AppName:     appName,
		ExeName:     info.ExeName,
		WindowTitle: info.Title,
		StartTime:   time.Now(),
		Date:        storage.Today(),
	}
}

func (t *Tracker) closeCurrentSession() {
	if t.currentSession == nil {
		return
	}

	now := time.Now()
	duration := int(now.Sub(t.currentSession.StartTime).Seconds())
	if duration < 1 {
		t.currentSession = nil
		return
	}

	session := &storage.Session{
		AppName:      t.currentSession.AppName,
		ExeName:      t.currentSession.ExeName,
		WindowTitle:  t.currentSession.WindowTitle,
		StartTime:    t.currentSession.StartTime,
		EndTime:      now,
		DurationSecs: duration,
		Date:         t.currentSession.Date,
	}

	t.pendingSessions = append(t.pendingSessions, session)
	t.currentSession = nil
}

func (t *Tracker) flushCurrentSession() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closeCurrentSession()
}

func (t *Tracker) flushPendingSessions() {
	t.mu.Lock()
	sessions := t.pendingSessions
	t.pendingSessions = nil
	t.mu.Unlock()

	for _, s := range sessions {
		if err := storage.InsertSession(s); err != nil {
			log.Printf("ERROR: failed to insert session: %v", err)
		}
		if err := storage.UpdateAppDaily(s.Date, s.AppName, s.ExeName, s.DurationSecs); err != nil {
			log.Printf("ERROR: failed to update app daily stats: %v", err)
		}

		if system.IsBrowser(s.ExeName) {
			cleanTitle := CleanWindowTitle(s.WindowTitle, s.ExeName)
			if err := storage.UpdateBrowserDaily(s.Date, cleanTitle, s.DurationSecs); err != nil {
				log.Printf("ERROR: failed to update browser daily stats: %v", err)
			}
		}
	}
}

var nameMap = map[string]string{
	"code":            "VS Code",
	"Code":            "VS Code",
	"devenv":          "Visual Studio",
	"idea64":          "IntelliJ IDEA",
	"pycharm64":       "PyCharm",
	"webstorm64":      "WebStorm",
	"goland64":        "GoLand",
	"rider64":         "Rider",
	"notepad++":       "Notepad++",
	"sublime_text":    "Sublime Text",
	"atom":            "Atom",
	"explorer":        "File Explorer",
	"slack":           "Slack",
	"Teams":           "Microsoft Teams",
	"WINWORD":         "Microsoft Word",
	"EXCEL":           "Microsoft Excel",
	"POWERPNT":        "PowerPoint",
	"OUTLOOK":         "Outlook",
	"Terminal":        "Windows Terminal",
	"WindowsTerminal": "Windows Terminal",
	"cmd":             "Command Prompt",
	"powershell":      "PowerShell",
	"pwsh":            "PowerShell",
	"wt":              "Windows Terminal",
}

func getAppName(exeName string) string {
	name := strings.TrimSuffix(exeName, ".exe")
	name = strings.TrimSuffix(name, ".EXE")

	if mapped, ok := nameMap[name]; ok {
		return mapped
	}

	if len(name) > 0 {
		first := strings.ToUpper(string(name[0]))
		if len(name) > 1 {
			return first + name[1:]
		}
		return first
	}

	return name
}

func (t *Tracker) startIPCOnport() {
	listener, err := net.Listen("tcp", IPCAddress)
	if err != nil {
		log.Printf("ERROR: IPC listener failed to bind on %s: %v", IPCAddress, err)
		return
	}
	defer listener.Close()

	go func() {
		<-t.ctx.Done()
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-t.ctx.Done():
				return
			default:
				continue
			}
		}
		go t.handleIPCConnection(conn)
	}
}

func (t *Tracker) handleIPCConnection(conn net.Conn) {
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Printf("WARN: IPC connection read failed: %v", err)
		return
	}

	cmd := string(buf[:n])
	switch cmd {
	case "stop":
		conn.Write([]byte("ok"))
		t.Stop()
	case "flush":
		t.flushCurrentSession()
		t.flushPendingSessions()
		t.persistActiveSession()
		conn.Write([]byte("ok"))
	default:
		log.Printf("WARN: Unknown IPC command received: %q", cmd)
	}
}
