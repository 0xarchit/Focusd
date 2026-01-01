package core

import (
	"context"
	"fmt"
	"focusd/storage"
	"focusd/system"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

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
	batchInterval   time.Duration
	pendingSessions []*storage.Session
	ctx             context.Context
	cancel          context.CancelFunc
}

func NewTracker() *Tracker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Tracker{
		pollInterval:  1 * time.Second,
		batchInterval: 10 * time.Second,
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

	pollTicker := time.NewTicker(t.pollInterval)
	batchTicker := time.NewTicker(t.batchInterval)
	persistTicker := time.NewTicker(30 * time.Second)
	retentionTicker := time.NewTicker(1 * time.Hour)
	focusTicker := time.NewTicker(5 * time.Second)
	defer pollTicker.Stop()
	defer batchTicker.Stop()
	defer persistTicker.Stop()
	defer retentionTicker.Stop()
	defer focusTicker.Stop()

	storage.EnforceRetention()

	var continuousUseStart time.Time
	breakReminderShown := false
	appLimitNotified := make(map[string]bool)

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
		case <-batchTicker.C:
			t.flushPendingSessions()
		case <-persistTicker.C:
			t.persistActiveSession()
		case <-retentionTicker.C:
			storage.EnforceRetention()
		case <-focusTicker.C:
			CheckPomodoroAndNotify()

			if system.GetBreakReminderEnabled() && !breakReminderShown && !continuousUseStart.IsZero() {
				mins := system.GetBreakReminderMinutes()
				if time.Since(continuousUseStart) >= time.Duration(mins)*time.Minute {
					ShowNotification("Break Reminder", fmt.Sprintf("You've been working for %d min. Take a break!", mins))
					breakReminderShown = true
				}
			}

			limits := system.GetAppTimeLimits()
			if len(limits) > 0 && t.currentSession != nil {
				exeName := strings.ToLower(t.currentSession.ExeName)
				if limit, ok := limits[exeName]; ok {
					if !appLimitNotified[exeName] {
						todayUsage := storage.GetAppUsageTodayMinutes(exeName)
						if todayUsage >= limit {
							ShowNotification("App Time Limit", t.currentSession.AppName+" has exceeded daily limit!")
							appLimitNotified[exeName] = true
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
	storage.InsertSession(recovered)
	storage.UpdateAppDaily(recovered.Date, recovered.AppName, recovered.ExeName, recovered.DurationSecs)
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
		if t.isSameSession(info.ExeName) {
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

func (t *Tracker) isSameSession(exeName string) bool {
	if t.currentSession == nil {
		return false
	}
	return t.currentSession.ExeName == exeName
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
		storage.InsertSession(s)
		storage.UpdateAppDaily(s.Date, s.AppName, s.ExeName, s.DurationSecs)

		if IsBrowser(s.ExeName) {
			cleanTitle := CleanWindowTitle(s.WindowTitle, s.ExeName)
			storage.UpdateBrowserDaily(s.Date, cleanTitle, s.DurationSecs)
		}
	}
}

func getAppName(exeName string) string {
	name := strings.TrimSuffix(exeName, ".exe")
	name = strings.TrimSuffix(name, ".EXE")

	nameMap := map[string]string{
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
		"Discord":         "Discord",
		"Spotify":         "Spotify",
		"slack":           "Slack",
		"Teams":           "Microsoft Teams",
		"Zoom":            "Zoom",
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
