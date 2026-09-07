package core

import (
	"bufio"
	"context"
	"fmt"
	"focusd/coreapi"
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

type activeSession struct {
	AppName     string
	ExeName     string
	WindowTitle string
	StartTime   time.Time
	Date        string
}

type Tracker struct {
	mu              sync.Mutex
	currentSession  *activeSession
	pollInterval    time.Duration
	pendingSessions []*storage.Session
	ctx             context.Context
	cancel          context.CancelFunc
	ipcToken        string
}

func NewTracker() *Tracker {
	ctx, cancel := context.WithCancel(context.Background())
	pollSeconds := storage.GetTrackingIntervalSeconds()
	if pollSeconds < 1 {
		pollSeconds = 1
	}
	token, err := coreapi.EnsureIPCToken()
	if err != nil {
		// Without a token the daemon still runs, but it will refuse every
		// state-changing IPC command, so `focusd stop` cannot reach it.
		log.Printf("ERROR: IPC token unavailable, remote control disabled: %v", err)
	}
	return &Tracker{
		pollInterval: time.Duration(pollSeconds) * time.Second,
		ctx:          ctx,
		cancel:       cancel,
		ipcToken:     token,
	}
}

func (t *Tracker) Start() error {
	listener, err := net.Listen("tcp", coreapi.IPCAddress)
	if err != nil {
		return err
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		t.Stop()
	}()

	if err := t.recoverOrphanedSession(); err != nil {
		return err
	}

	go t.startIPCOnPort(listener)

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
			return nil
		case <-pollTicker.C:
			if storage.IsPaused() {
				t.flushCurrentSession()
				stateMu.Lock()
				continuousUseStart = time.Time{}
				stateMu.Unlock()
				continue
			}
			t.poll()
			t.mu.Lock()
			t.rotateActiveSessionAtMidnight()
			t.mu.Unlock()
			t.persistActiveSession()
			stateMu.Lock()
			t.mu.Lock()
			hasActive := t.currentSession != nil
			t.mu.Unlock()
			if !hasActive {
				continuousUseStart = time.Time{}
			} else if continuousUseStart.IsZero() {
				continuousUseStart = time.Now()
			}
			stateMu.Unlock()
		case <-persistTicker.C:
			t.flushPendingSessions()
			t.persistActiveSession()
		case <-retentionTicker.C:
			storage.EnforceRetention()
		case <-focusTicker.C:
			checkPomodoroAndNotify()

			today := storage.Today()
			now := time.Now()
			snoozeDuration := time.Duration(system.GetSnoozeDurationMinutes()) * time.Minute

			stateMu.Lock()
			breakSnoozed := !breakSnoozedUntil.IsZero() && now.Before(breakSnoozedUntil)
			continuousUseActive := !continuousUseStart.IsZero()
			var elapsed time.Duration
			if continuousUseActive {
				elapsed = time.Since(continuousUseStart)
			}
			stateMu.Unlock()

			if system.GetBreakReminderEnabled() && !breakSnoozed && continuousUseActive {
				mins := system.GetBreakReminderMinutes()
				if elapsed >= time.Duration(mins)*time.Minute {
					stateMu.Lock()
					continuousUseStart = time.Now() // Reset immediately to prevent repeated triggers
					stateMu.Unlock()

					showNotificationWithAction("Break Reminder",
						fmt.Sprintf("You've been working for %d min. Take a break!", mins),
						func(disable bool) {
							if disable {
								stateMu.Lock()
								breakSnoozedUntil = time.Now().Add(snoozeDuration)
								stateMu.Unlock()
							}
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

			// Reset prevSessionApp guard when user switches away
			if sessionExe != prevSessionApp {
				prevSessionApp = ""
			}

			if len(limits) > 0 && sessionExe != "" {
				stateMu.Lock()
				snoozeTime := disabledLimitApps[sessionExe]
				appSnoozed := !snoozeTime.IsZero() && now.Before(snoozeTime)
				if !snoozeTime.IsZero() && !now.Before(snoozeTime) {
					disabledLimitApps[sessionExe] = time.Time{}
					if prevSessionApp == sessionExe {
						prevSessionApp = ""
					}
					appSnoozed = false
				}
				stateMu.Unlock()

				if limit, ok := limits[sessionExe]; ok && !appSnoozed && prevSessionApp != sessionExe {
					// GetAppUsageTodaySeconds already includes the live active session elapsed time
					todayUsageSeconds := storage.GetAppUsageTodaySeconds(sessionExe)
					todayUsage := todayUsageSeconds / 60
					if todayUsage >= limit {
						prevSessionApp = sessionExe // Set guard immediately to prevent repeated triggers
						exeCopy := sessionExe
						showNotificationWithAction("App Time Limit",
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

func (t *Tracker) Stop() {
	t.cancel()
}

func (t *Tracker) recoverOrphanedSession() error {
	recovered, err := storage.RecoverActiveSession()
	if err != nil {
		return fmt.Errorf("failed to recover active session from storage: %w", err)
	}
	if recovered == nil {
		return nil
	}
	if err := storage.InsertSessionWithDaily(recovered, ""); err != nil {
		return fmt.Errorf("failed to recover orphaned session: %w", err)
	}
	storage.ClearActiveSession()
	return nil
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
	if err := storage.SaveActiveSession(record); err != nil {
		log.Printf("ERROR: failed to save active session: %v", err)
	}
}

// pollObservation is one reading of the foreground window, decoupled from the
// Win32 calls so session decisions can be tested directly.
type pollObservation struct {
	exeName string
	title   string
}

func (t *Tracker) poll() {
	info, err := system.GetForegroundWindowInfo()
	if err != nil || info == nil || info.ExeName == "" {
		t.mu.Lock()
		if t.currentSession != nil {
			t.closeCurrentSession()
		}
		t.mu.Unlock()
		return
	}

	if system.IsWhitelisted(info.ExeName) {
		t.mu.Lock()
		if t.currentSession != nil {
			t.closeCurrentSession()
		}
		t.mu.Unlock()
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.applyPoll(&pollObservation{exeName: info.ExeName, title: info.Title})
}

// applyPoll updates session state from one observation. Caller holds t.mu.
func (t *Tracker) applyPoll(obs *pollObservation) {
	if t.currentSession != nil {
		isSameExe := t.currentSession.ExeName == obs.exeName

		// A blank caption means a child window of the app we are already
		// tracking took focus. Closing here would split one session into many
		// and inflate open counts for no user-visible change.
		if isSameExe && obs.title == "" {
			return
		}

		isBrowser := system.IsBrowser(obs.exeName)
		isSameTitle := t.currentSession.WindowTitle == obs.title

		if isSameExe && (!isBrowser || isSameTitle) {
			return
		}
		t.closeCurrentSession()
	}

	if obs.title == "" {
		// No previous session to attach to and nothing to record a title for.
		return
	}

	t.currentSession = &activeSession{
		AppName:     getAppName(obs.exeName),
		ExeName:     obs.exeName,
		WindowTitle: obs.title,
		StartTime:   time.Now(),
		Date:        storage.Today(),
	}
}

func (t *Tracker) closeCurrentSession() {
	if t.currentSession == nil {
		return
	}

	now := time.Now()
	slices := splitSessionAtMidnight(t.currentSession, now)
	t.pendingSessions = append(t.pendingSessions, slices...)
	t.currentSession = nil
}

// splitSessionAtMidnight breaks a session that crosses local midnight into one
// slice per day. Without this the whole session was stamped with the date it
// started on, so time after midnight was attributed to yesterday. A session can
// span more than one boundary, for example after a crash or a resume from
// sleep, so this walks every midnight rather than just the most recent one.
func splitSessionAtMidnight(s *activeSession, now time.Time) []*storage.Session {
	if s == nil || !now.After(s.StartTime) {
		return nil
	}

	var out []*storage.Session
	cursor := s.StartTime

	for {
		midnight := startOfDayAfter(cursor)
		if !midnight.Before(now) {
			break
		}
		if duration := int(midnight.Sub(cursor).Seconds()); duration >= 1 {
			out = append(out, &storage.Session{
				AppName:      s.AppName,
				ExeName:      s.ExeName,
				WindowTitle:  s.WindowTitle,
				StartTime:    cursor,
				EndTime:      midnight,
				DurationSecs: duration,
				Date:         dateStr(cursor),
			})
		}
		cursor = midnight
	}

	if duration := int(now.Sub(cursor).Seconds()); duration >= 1 {
		out = append(out, &storage.Session{
			AppName:      s.AppName,
			ExeName:      s.ExeName,
			WindowTitle:  s.WindowTitle,
			StartTime:    cursor,
			EndTime:      now,
			DurationSecs: duration,
			Date:         dateStr(cursor),
		})
	}

	return out
}

// startOfDayAfter returns the first local midnight strictly after t.
func startOfDayAfter(t time.Time) time.Time {
	midnight := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	if !midnight.After(t) {
		midnight = midnight.AddDate(0, 0, 1)
	}
	return midnight
}

func dateStr(t time.Time) string {
	return t.Format("2006-01-02")
}

// rotateActiveSessionAtMidnight is the single place a session is rebased across
// local midnight. Read paths no longer rotate, so this runs once per poll
// rather than once per reader.
func (t *Tracker) rotateActiveSessionAtMidnight() {
	if t.currentSession == nil {
		return
	}

	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if !t.currentSession.StartTime.Before(midnight) {
		return
	}

	// Write the elapsed slice for the previous day before rebasing, so the two
	// halves are recorded exactly once.
	slices := splitSessionAtMidnight(t.currentSession, now)
	priorOnly := slices[:0]
	for _, s := range slices {
		if s.Date == t.currentSession.Date {
			priorOnly = append(priorOnly, s)
		}
	}
	t.pendingSessions = append(t.pendingSessions, priorOnly...)

	t.currentSession.StartTime = midnight
	t.currentSession.Date = midnight.Format("2006-01-02")

	if err := storage.RebaseActiveSessionStart(midnight, t.currentSession.Date); err != nil {
		log.Printf("ERROR: failed to rebase active session at midnight: %v", err)
	}
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
		cleanBrowserTitle := ""
		if system.IsBrowser(s.ExeName) {
			cleanBrowserTitle = cleanWindowTitle(s.WindowTitle, s.ExeName)
		}
		if err := storage.InsertSessionWithDaily(s, cleanBrowserTitle); err != nil {
			log.Printf("ERROR: failed to insert session with daily (exe: %s, date: %s, start: %s): %v",
				s.ExeName, s.Date, s.StartTime.Format(time.RFC3339), err)
			t.mu.Lock()
			t.pendingSessions = append(t.pendingSessions, s)
			t.mu.Unlock()
		}
	}
}

var nameMap = map[string]string{
	"code":            "VS Code",
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
	"teams":           "Microsoft Teams",
	"winword":         "Microsoft Word",
	"excel":           "Microsoft Excel",
	"powerpnt":        "PowerPoint",
	"outlook":         "Outlook",
	"terminal":        "Windows Terminal",
	"windowsterminal": "Windows Terminal",
	"cmd":             "Command Prompt",
	"powershell":      "PowerShell",
	"pwsh":            "PowerShell",
	"wt":              "Windows Terminal",
}

func getAppName(exeName string) string {
	name := strings.TrimSuffix(exeName, ".exe")
	name = strings.TrimSuffix(name, ".EXE")

	if mapped, ok := nameMap[strings.ToLower(name)]; ok {
		return mapped
	}

	if len(name) > 0 {
		first := strings.ToUpper(string(name[0]))
		if len(name) > 1 {
			return first + name[1:]
		}
		return first
	}

	return exeName
}

func (t *Tracker) startIPCOnPort(listener net.Listener) {
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
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}
		go t.handleIPCConnection(conn)
	}
}

func (t *Tracker) handleIPCConnection(conn net.Conn) {
	defer conn.Close()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		log.Printf("WARN: IPC set deadline failed: %v", err)
		return
	}
	reader := bufio.NewReader(conn)
	raw, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("WARN: IPC connection read failed: %v", err)
		return
	}

	cmd, token, err := coreapi.ParseIPCCommand(raw)
	if err != nil {
		log.Printf("WARN: IPC command rejected: %v", err)
		return
	}

	if coreapi.IPCCommandNeedsToken(cmd) {
		if !coreapi.TokensMatch(token, t.ipcToken) {
			// No reply, so a caller cannot distinguish a bad token from an
			// unknown command or a closed port.
			log.Printf("WARN: IPC command refused: bad token")
			return
		}
	}

	switch cmd {
	case "stop":
		t.flushCurrentSession()
		t.flushPendingSessions()
		t.persistActiveSession()
		if _, err := conn.Write([]byte("ok\n")); err != nil {
			log.Printf("WARN: IPC write failed: %v", err)
		}
		t.Stop()
	case "flush":
		t.flushCurrentSession()
		t.flushPendingSessions()
		t.persistActiveSession()
		if _, err := conn.Write([]byte("ok\n")); err != nil {
			log.Printf("WARN: IPC write failed: %v", err)
		}
	default:
		log.Printf("WARN: Unknown IPC command received")
	}
}
