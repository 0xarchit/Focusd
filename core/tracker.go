package core

import (
	"bufio"
	"context"
	"fmt"
	"focusd/coreapi"
	"focusd/storage"
	"focusd/system"
	"io"
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
	ExePath     string
	AppID       string
	WindowTitle string
	StartTime   time.Time
	Date        string
}

// gapFactor is how much later than one poll interval a tick may arrive before
// it is treated as a sleep or hibernate rather than ordinary jitter.
const gapFactor = 3

// persistInterval is the shortest gap between two identical active-session
// writes.
const persistInterval = 30 * time.Second

type Tracker struct {
	mu              sync.Mutex
	currentSession  *activeSession
	pollInterval    time.Duration
	pendingSessions []*storage.Session
	ctx             context.Context
	cancel          context.CancelFunc
	ipcToken        string
	lastPollAt      time.Time
	lastPersistAt   time.Time

	persistedAppName     string
	persistedExeName     string
	persistedWindowTitle string
	persistedStartTime   time.Time
	persistedDate        string

	// persistWrites counts completed active-session writes. Comparing counts
	// avoids depending on clock resolution, which on Windows is coarse enough
	// that two writes can share one tick.
	persistWrites int
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
			clearActiveSessionLogged()
			// Tear down any in-flight toast process with the daemon.
			cancelNotify()
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
			// Force a write on identity change so an unexpected termination
			// recovers the correct app.
			forcePersist := t.needsPersist()
			t.mu.Unlock()
			if forcePersist {
				t.forcePersistActiveSession()
			} else {
				t.persistActiveSession()
			}
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
			CheckPomodoroCompletion()

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
					// Reset the anchor only once the toast actually goes out:
					// resetting it before a throttled notify dropped the
					// reminder for another full interval.
					if showNotificationWithAction(notifyContext, "Break Reminder",
						fmt.Sprintf("You've been working for %d min. Take a break!", mins),
						func(disable bool) {
							if disable {
								stateMu.Lock()
								breakSnoozedUntil = time.Now().Add(snoozeDuration)
								stateMu.Unlock()
							}
						}) {
						stateMu.Lock()
						continuousUseStart = time.Now()
						stateMu.Unlock()
					}
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
			var sessionExe, sessionAppName, sessionAppID string
			if t.currentSession != nil {
				sessionExe = strings.ToLower(t.currentSession.ExeName)
				sessionAppName = t.currentSession.AppName
				sessionAppID = t.currentSession.AppID
			}
			t.mu.Unlock()

			if sessionExe != prevSessionApp {
				prevSessionApp = ""
			}

			if len(limits) > 0 && sessionExe != "" {
				// Limits are configured against an app key. A bare exe name is
				// a wildcard across installs, so existing settings keep firing.
				limitKey, hasLimit := system.MatchLimitKey(limits, sessionAppID, sessionExe)

				stateMu.Lock()
				snoozeTime := disabledLimitApps[limitKey]
				appSnoozed := !snoozeTime.IsZero() && now.Before(snoozeTime)
				if !snoozeTime.IsZero() && !now.Before(snoozeTime) {
					disabledLimitApps[limitKey] = time.Time{}
					if prevSessionApp == limitKey {
						prevSessionApp = ""
					}
					appSnoozed = false
				}
				stateMu.Unlock()

				if limit, ok := limits[limitKey]; ok && hasLimit && !appSnoozed && prevSessionApp != limitKey {
					// GetAppUsageTodaySeconds already includes the live active session elapsed time
					todayUsageSeconds := storage.GetAppUsageTodaySeconds(sessionAppID)
					todayUsage := todayUsageSeconds / 60
					if todayUsage >= limit {
						keyCopy := limitKey
						// Arm the guard only once the toast actually goes out:
						// arming it before a throttled notify dropped the alert
						// until the app was switched away and back.
						if showNotificationWithAction(notifyContext, "App Time Limit",
							sessionAppName+" has exceeded daily limit!",
							func(disable bool) {
								if disable {
									stateMu.Lock()
									disabledLimitApps[keyCopy] = time.Now().Add(snoozeDuration)
									stateMu.Unlock()
								}
							}) {
							prevSessionApp = limitKey
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

// clearActiveSessionLogged removes the persisted active-session row, retrying
// briefly and logging when it still fails: an un-cleared row whose interval was
// already flushed to history would otherwise be re-inserted by recovery on the
// next start and bill the same time twice.
func clearActiveSessionLogged() {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if err = storage.ClearActiveSession(); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	log.Printf("ERROR: failed to clear active session: %v", err)
}

func (t *Tracker) recoverOrphanedSession() error {
	recovered, err := storage.RecoverActiveSession()
	if err != nil {
		return fmt.Errorf("failed to recover active session from storage: %w", err)
	}
	if recovered == nil {
		return nil
	}

	// Recompute the browser title so recovered sessions keep their site totals.
	cleanTitle := ""
	if system.IsBrowser(recovered.ExeName) {
		cleanTitle = cleanWindowTitle(recovered.WindowTitle, recovered.ExeName)
	}

	for _, segment := range splitRecoveredAtMidnight(recovered) {
		alreadyStored, err := storage.SessionExists(segment)
		if err != nil {
			return fmt.Errorf("failed to check for a stored copy of the session: %w", err)
		}
		if alreadyStored {
			// The interval reached history before the active row survived a
			// failed shutdown clear; inserting again would bill the time twice.
			log.Printf("WARN: recovered session is already in history; skipping insert")
			continue
		}
		if err := storage.InsertSessionWithDaily(segment, cleanTitle); err != nil {
			return fmt.Errorf("failed to recover orphaned session: %w", err)
		}
	}
	if err := storage.ClearActiveSession(); err != nil {
		return fmt.Errorf("failed to clear recovered active session: %w", err)
	}
	return nil
}

// splitRecoveredAtMidnight splits an orphaned interval at local-midnight
// boundaries, so a session that was still active when the machine crashed
// across midnight is billed to the days it actually covers.
func splitRecoveredAtMidnight(s *storage.Session) []*storage.Session {
	if s.EndTime.IsZero() || !s.EndTime.After(s.StartTime) {
		return []*storage.Session{s}
	}

	var out []*storage.Session
	start := s.StartTime
	date := s.Date
	for {
		nextMidnight := time.Date(start.Year(), start.Month(), start.Day()+1, 0, 0, 0, 0, time.Local)
		if !s.EndTime.After(nextMidnight) {
			segment := *s
			segment.StartTime = start
			segment.Date = date
			segment.DurationSecs = int(s.EndTime.Sub(start).Seconds())
			out = append(out, &segment)
			return out
		}

		segment := *s
		segment.StartTime = start
		segment.EndTime = nextMidnight
		segment.Date = date
		segment.DurationSecs = int(nextMidnight.Sub(start).Seconds())
		out = append(out, &segment)

		start = nextMidnight
		date = nextMidnight.Format("2006-01-02")
	}
}

// needsPersist reports whether the in-memory session differs from what was last
// written. Caller holds t.mu.
func (t *Tracker) needsPersist() bool {
	if t.currentSession == nil {
		return false
	}
	return t.currentSession.AppName != t.persistedAppName ||
		t.currentSession.ExeName != t.persistedExeName ||
		t.currentSession.WindowTitle != t.persistedWindowTitle ||
		!t.currentSession.StartTime.Equal(t.persistedStartTime) ||
		t.currentSession.Date != t.persistedDate
}

// persistActiveSession writes the active session unless nothing changed and the
// throttle window has not elapsed. lastSeen still advances on the slow tick, so
// orphan recovery does not bill a crash as active time.
func (t *Tracker) persistActiveSession() {
	t.writeActiveSession(false)
}

func (t *Tracker) forcePersistActiveSession() {
	t.writeActiveSession(true)
}

// writeActiveSession snapshots the session under t.mu and performs the SQLite
// write outside it: a busy or slow disk must not stall the poll loop or the
// IPC handlers queued on the mutex.
func (t *Tracker) writeActiveSession(force bool) {
	t.mu.Lock()
	if t.currentSession == nil {
		t.mu.Unlock()
		clearActiveSessionLogged()
		return
	}

	now := time.Now()
	changed := t.needsPersist()
	if !force && !changed && now.Sub(t.lastPersistAt) < persistInterval {
		t.mu.Unlock()
		return
	}

	record := &storage.ActiveSessionRecord{
		AppName:     t.currentSession.AppName,
		ExeName:     t.currentSession.ExeName,
		ExePath:     t.currentSession.ExePath,
		AppID:       t.currentSession.AppID,
		WindowTitle: t.currentSession.WindowTitle,
		StartTime:   t.currentSession.StartTime,
		LastSeen:    now,
		Date:        t.currentSession.Date,
	}
	t.mu.Unlock()

	if err := storage.SaveActiveSession(record); err != nil {
		log.Printf("ERROR: failed to save active session: %v", err)
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	// Only mark as persisted when the same session is still active; a newer
	// session started mid-write sets its own bookkeeping on its first persist.
	if t.currentSession != nil &&
		t.currentSession.StartTime.Equal(record.StartTime) &&
		t.currentSession.AppName == record.AppName {
		t.persistedAppName = record.AppName
		t.persistedExeName = record.ExeName
		t.persistedWindowTitle = record.WindowTitle
		t.persistedStartTime = record.StartTime
		t.persistedDate = record.Date
	}
	t.lastPersistAt = now
	t.persistWrites++
}

// readIdleSeconds is a seam over the Win32 idle clock so session accounting
// can be tested without real keyboard state.
var readIdleSeconds = system.GetIdleSeconds

// idleFor reports whether the user has been away long enough that time should
// stop accruing, and for how long they have been idle.
func (t *Tracker) idleFor() (bool, time.Duration) {
	if system.GetTrackWhileIdle() {
		return false, 0
	}

	threshold := system.GetIdleThresholdMinutes()
	if threshold <= 0 {
		return false, 0
	}

	idleSeconds, err := readIdleSeconds()
	if err != nil {
		// If the input state cannot be read, keep tracking rather than
		// silently dropping time.
		return false, 0
	}
	if idleSeconds < threshold*60 {
		return false, 0
	}
	return true, time.Duration(idleSeconds) * time.Second
}

// detectWallClockGap closes the running session when the wall clock jumped much
// further than one poll interval, which means the machine slept or hibernated.
func (t *Tracker) detectWallClockGap() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	if t.lastPollAt.IsZero() {
		t.lastPollAt = now
		return
	}

	prev := t.lastPollAt
	gap := now.Sub(prev)
	t.lastPollAt = now

	if t.currentSession == nil {
		return
	}
	if gap > gapFactor*t.pollInterval {
		// End the session at the last poll, not at now: the sleep interval
		// itself is not time the user spent in the app.
		log.Printf("WARN: wall clock gap of %s detected, closing session", gap)
		t.closeCurrentSessionAt(prev)
	}
}

// pollObservation is one reading of the foreground window, decoupled from the
// Win32 calls so session decisions can be tested directly.
type pollObservation struct {
	exeName string
	exePath string
	appID   string
	title   string
}

func (t *Tracker) poll() {
	t.detectWallClockGap()

	// Idle time must not accrue: close the session at the moment the user
	// stopped interacting, so the away stretch is never billed. Returning to
	// any window then starts a fresh session.
	if idle, idleFor := t.idleFor(); idle {
		t.mu.Lock()
		t.closeCurrentSessionAt(time.Now().Add(-idleFor))
		t.mu.Unlock()
		return
	}

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

	t.applyPoll(&pollObservation{
		exeName: info.ExeName,
		exePath: info.ExePath,
		appID:   info.AppID,
		title:   info.Title,
	})
}

// applyPoll updates session state from one observation. Caller holds t.mu.
func (t *Tracker) applyPoll(obs *pollObservation) {
	// Compute the stored form once: the comparison below must see the same
	// text that gets persisted, or a private or redacted title looks changed
	// on every poll and splits the session.
	stored := storableTitle(obs.title)
	if stored == "" && IsPrivateWindowTitle(obs.title) {
		stored = privateWindowLabel(obs.exeName)
	}

	if t.currentSession != nil {
		// Identity, not the exe base name, decides whether this is the same
		// app. Firefox and Firefox Nightly share a base name.
		isSameApp := t.currentSession.AppID == obs.appID &&
			(t.currentSession.ExeName == obs.exeName || obs.appID == "")

		// A blank caption means a child window of the app we are already
		// tracking took focus. Closing here would split one session into many
		// and inflate open counts for no user-visible change.
		if isSameApp && obs.title == "" {
			return
		}

		isBrowser := system.IsBrowser(obs.exeName)

		// Browsers compare on the cleaned title: a raw title ticks with notification
		// counts and tab-group noise, which would split the session repeatedly.
		isSameTitle := t.currentSession.WindowTitle == stored
		if isBrowser && isSameApp {
			isSameTitle = cleanWindowTitle(t.currentSession.WindowTitle, obs.exeName) ==
				cleanWindowTitle(stored, obs.exeName)
		}

		if isSameApp && (!isBrowser || isSameTitle) {
			return
		}
		t.closeCurrentSession()
	}

	if stored == "" {
		// No previous session to attach to and nothing to record a title for.
		return
	}

	identity := system.ResolveIdentity(obs.exeName, obs.exePath)
	displayName := system.ResolveDisplayName(identity.AppID, identity.ExeName, identity.DisplayName)

	t.currentSession = &activeSession{
		AppName:     displayName,
		ExeName:     obs.exeName,
		ExePath:     obs.exePath,
		AppID:       obs.appID,
		WindowTitle: stored,
		StartTime:   time.Now(),
		Date:        storage.Today(),
	}
}

// privateWindowLabel is the placeholder stored for a private browsing session.
func privateWindowLabel(exeName string) string {
	base := baseExeName(exeName)
	if base == "" {
		return "Private Browsing"
	}
	return "Private Browsing (" + base + ")"
}

func (t *Tracker) closeCurrentSession() {
	t.closeCurrentSessionAt(time.Now())
}

// closeCurrentSessionAt ends the running session at end rather than at the wall
// clock, so callers can exclude time that was not real use: an idle stretch or
// a sleep gap. Caller holds t.mu.
func (t *Tracker) closeCurrentSessionAt(end time.Time) {
	if t.currentSession == nil {
		return
	}
	if end.Before(t.currentSession.StartTime) {
		// Never bill negative time; an idle clock that predates the session
		// start must not drop the session entirely.
		end = t.currentSession.StartTime
	}
	slices := splitSessionAtMidnight(t.currentSession, end)
	t.pendingSessions = append(t.pendingSessions, slices...)
	t.currentSession = nil
}

// splitSessionAtMidnight breaks a session into one slice per calendar day.
// Every boundary is walked, not just the most recent, because a session can span
// several days after a crash or a resume from sleep.
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
				ExePath:      s.ExePath,
				AppID:        s.AppID,
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
			ExePath:      s.ExePath,
			AppID:        s.AppID,
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

// rotateActiveSessionAtMidnight is the single writer for rebasing a session
// across local midnight.
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

// flushForRead persists what a read needs without closing the live session.
// The IPC flush command backs `focusd status`, `focusd stats`, and the TUI
// refresh, which only want current data on disk; closing the session here
// fragmented it, bumped open counts, and dropped the active_session row until
// the next poll. Closing a session is stop's job.
func (t *Tracker) flushForRead() {
	t.flushPendingSessions()
	t.persistActiveSession()
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

// maxIPCLineBytes bounds a single IPC request line. Real commands are far
// smaller; the cap stops an unauthenticated peer from streaming bytes without
// a newline and growing the read buffer within the deadline.
const maxIPCLineBytes = 4 * 1024

func (t *Tracker) handleIPCConnection(conn net.Conn) {
	defer conn.Close()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		log.Printf("WARN: IPC set deadline failed: %v", err)
		return
	}
	// The line is read before the token is checked, so the read must be bound
	// up front: without it, any local process could make this buffer grow to
	// gigabytes within the deadline.
	reader := bufio.NewReader(io.LimitReader(conn, maxIPCLineBytes))
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
		t.flushForRead()
		if _, err := conn.Write([]byte("ok\n")); err != nil {
			log.Printf("WARN: IPC write failed: %v", err)
		}
	default:
		log.Printf("WARN: Unknown IPC command received")
	}
}
