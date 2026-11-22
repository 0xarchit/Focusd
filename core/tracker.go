package core

import (
	"context"
	"fmt"
	"focusd/coreapi"
	"focusd/storage"
	"focusd/system"
	"log"
	"net"
	"sync"
	"time"
)

type activeSession struct {
	calendar    *sessionCalendar
	SessionID   string
	SliceID     string
	SiteName    string
	SiteKnown   bool
	Generation  uint64
	PrivacyMode string
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
	writeCheckpoint func([]*storage.Session, *storage.ActiveSessionRecord) error
	ipcHandlers     sync.WaitGroup
	ipcDone         chan struct{}
	calendarFactory func() (*sessionCalendar, error)
	calendarErr     error
	controller      *daemonController
	checkpointMu    sync.Mutex
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
		calendarFactory: captureWindowsCalendar,
		pollInterval:    time.Duration(pollSeconds) * time.Second,
		ctx:             ctx,
		cancel:          cancel,
		ipcToken:        token,
	}
}

func (t *Tracker) Start() error {
	return t.startControlled()
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

	if !recovered.SiteKnown {
		classified := ClassifyObservation(recovered.ExeName, recovered.ExePath, recovered.WindowTitle)
		if recovered.WindowTitle == privateWindowLabel(recovered.ExeName) {
			classified.StoredTitle = "[redacted]"
			classified.SiteName = ""
		}
		recovered.WindowTitle, recovered.SiteName, recovered.SiteKnown = classified.StoredTitle, classified.SiteName, true
	}
	var sessions []*storage.Session
	segments := splitRecoveredAtMidnight(recovered)
	if t.calendarFactory != nil {
		calendar, err := t.calendarFactory()
		if err != nil {
			t.calendarErr = err
			return err
		}
		segments, err = splitSessionWithCalendar(&activeSession{SessionID: recovered.SessionID, SliceID: recovered.SliceID, SiteName: recovered.SiteName, SiteKnown: recovered.SiteKnown, Generation: recovered.Generation, AppName: recovered.AppName, ExeName: recovered.ExeName, ExePath: recovered.ExePath, AppID: recovered.AppID, WindowTitle: recovered.WindowTitle, StartTime: recovered.StartTime, Date: recovered.Date}, recovered.EndTime, calendar)
		if err != nil {
			t.calendarErr = err
			return err
		}
	}
	for _, segment := range segments {
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
		sessions = append(sessions, segment)
	}
	return storage.WriteCheckpoint(sessions, nil)
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
			segment.SliceID = sliceID(s.SessionID, start)
			segment.Date = date
			segment.DurationSecs = int(s.EndTime.Sub(start).Seconds())
			out = append(out, &segment)
			return out
		}

		segment := *s
		segment.StartTime = start
		segment.SliceID = sliceID(s.SessionID, start)
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
	t.checkpointMu.Lock()
	defer t.checkpointMu.Unlock()
	t.mu.Lock()
	if t.calendarErr != nil {
		t.mu.Unlock()
		return
	}

	now := time.Now()
	changed := t.needsPersist()
	if !force && len(t.pendingSessions) == 0 && t.currentSession != nil && !changed && now.Sub(t.lastPersistAt) < persistInterval {
		t.mu.Unlock()
		return
	}
	var record *storage.ActiveSessionRecord
	if t.currentSession != nil {
		record = &storage.ActiveSessionRecord{
			SessionID:   t.currentSession.SessionID,
			SliceID:     t.currentSession.SliceID,
			SiteName:    t.currentSession.SiteName,
			SiteKnown:   t.currentSession.SiteKnown,
			Generation:  t.currentSession.Generation,
			AppName:     t.currentSession.AppName,
			ExeName:     t.currentSession.ExeName,
			ExePath:     t.currentSession.ExePath,
			AppID:       t.currentSession.AppID,
			WindowTitle: t.currentSession.WindowTitle,
			StartTime:   t.currentSession.StartTime,
			LastSeen:    now,
			Date:        t.currentSession.Date,
		}
	}
	originals := append([]*storage.Session(nil), t.pendingSessions...)
	sessions := make([]*storage.Session, len(originals))
	for i, session := range originals {
		if session != nil {
			clone := *session
			sessions[i] = &clone
		}
	}
	active := t.currentSession
	write := t.writeCheckpoint
	t.mu.Unlock()
	for _, session := range sessions {
		if session != nil && !session.SiteKnown {
			classified := ClassifyObservation(session.ExeName, session.ExePath, session.WindowTitle)
			session.WindowTitle, session.SiteName, session.SiteKnown = classified.StoredTitle, classified.SiteName, true
		}
	}
	if write == nil {
		write = storage.WriteCheckpoint
	}
	if err := write(sessions, record); err != nil {
		log.Printf("ERROR: failed to save active session: %v", err)
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	// Direct helper callers may append while SQLite is busy. Remove only the
	// snapshot that committed; preserve newer pending intervals and identities.
	prefixMatches := len(t.pendingSessions) >= len(originals)
	for i, original := range originals {
		if !prefixMatches || t.pendingSessions[i] != original {
			prefixMatches = false
			break
		}
	}
	if prefixMatches {
		t.pendingSessions = t.pendingSessions[len(originals):]
	}
	// Only mark as persisted when the same session is still active; a newer
	// session started mid-write sets its own bookkeeping on its first persist.
	if record != nil && t.currentSession == active && t.currentSession != nil &&
		t.currentSession.SessionID == record.SessionID && t.currentSession.SliceID == record.SliceID &&
		t.currentSession.Generation == record.Generation &&
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
	if t.controller == nil {
		t.detectWallClockGap()
	}
	if t.controller != nil {
		state, err := system.ReadSessionState()
		if err != nil || !state.Active() {
			t.controller.reminders.setPresent(time.Now(), false)
			t.mu.Lock()
			t.closeCurrentSession()
			t.mu.Unlock()
			return
		}
		t.controller.reminders.setPresent(time.Now(), true)
	}

	// Idle time must not accrue: close the session at the moment the user
	// stopped interacting, so the away stretch is never billed. Returning to
	// any window then starts a fresh session.
	if idle, idleFor := t.idleFor(); idle {
		if t.controller != nil {
			t.controller.reminders.setPresent(time.Now(), false)
		}
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
	classified := ClassifyObservation(obs.exeName, obs.exePath, obs.title)
	stored := classified.StoredTitle

	if t.currentSession != nil {
		policyChanged := t.currentSession.PrivacyMode != "" && t.currentSession.PrivacyMode != system.GetPrivacyMode()
		// Identity, not the exe base name, decides whether this is the same
		// app. Firefox and Firefox Nightly share a base name.
		isSameApp := t.currentSession.AppID == obs.appID &&
			(t.currentSession.ExeName == obs.exeName || obs.appID == "")

		// A blank caption means a child window of the app we are already
		// tracking took focus. Closing here would split one session into many
		// and inflate open counts for no user-visible change.
		if isSameApp && obs.title == "" && !policyChanged {
			return
		}
		if isSameApp && obs.title == "" && policyChanged {
			classified = ClassifyObservation(obs.exeName, obs.exePath, t.currentSession.WindowTitle)
			stored = classified.StoredTitle
		}

		isBrowser := classified.Browser

		// Browsers compare on the cleaned title: a raw title ticks with notification
		// counts and tab-group noise, which would split the session repeatedly.
		isSameTitle := t.currentSession.WindowTitle == stored
		if isBrowser && isSameApp {
			if !t.currentSession.SiteKnown {
				previous := ClassifyObservation(t.currentSession.ExeName, t.currentSession.ExePath, t.currentSession.WindowTitle)
				t.currentSession.SiteName, t.currentSession.SiteKnown = previous.SiteName, true
			}
			isSameTitle = t.currentSession.SiteName == classified.SiteName && cleanWindowTitle(t.currentSession.WindowTitle, obs.exeName) ==
				cleanWindowTitle(stored, obs.exeName)
		}

		if isSameApp && !policyChanged && (!isBrowser || isSameTitle) {
			return
		}
		t.closeCurrentSession()
		if t.calendarErr != nil {
			return
		}
	}

	if stored == "" {
		// No previous session to attach to and nothing to record a title for.
		return
	}

	identity := system.ResolveIdentity(obs.exeName, obs.exePath)
	displayName := system.ResolveDisplayName(identity.AppID, identity.ExeName, identity.DisplayName)

	sessionID, err := randomClockID()
	if err != nil {
		log.Printf("ERROR: session identity unavailable: %v", err)
		return
	}
	generation, err := storage.GetDataGeneration()
	if err != nil {
		log.Printf("ERROR: data generation unavailable: %v", err)
		return
	}
	started := time.Now()
	var calendar *sessionCalendar
	date := dateStr(started)
	if t.calendarFactory != nil {
		calendar, err = t.calendarFactory()
		if err == nil {
			date, err = calendar.date(started)
		}
		if err != nil {
			t.calendarErr = err
			return
		}
	}
	t.currentSession = &activeSession{
		calendar:    calendar,
		PrivacyMode: system.GetPrivacyMode(),
		SessionID:   sessionID, SliceID: sliceID(sessionID, started), SiteName: classified.SiteName, SiteKnown: true, Generation: generation,
		AppName:     displayName,
		ExeName:     obs.exeName,
		ExePath:     obs.exePath,
		AppID:       obs.appID,
		WindowTitle: stored,
		StartTime:   started,
		Date:        date,
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
	var slices []*storage.Session
	if t.currentSession.calendar != nil {
		var err error
		slices, err = splitSessionWithCalendar(t.currentSession, end, t.currentSession.calendar)
		if err != nil {
			t.calendarErr = err
			return
		}
	} else {
		slices = splitSessionAtMidnight(t.currentSession, end)
	}
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
				SessionID: s.SessionID, SliceID: sliceID(s.SessionID, cursor), SiteName: s.SiteName, SiteKnown: s.SiteKnown, Generation: s.Generation,
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
			SessionID: s.SessionID, SliceID: sliceID(s.SessionID, cursor), SiteName: s.SiteName, SiteKnown: s.SiteKnown, Generation: s.Generation,
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
	date := dateStr(midnight)
	if t.currentSession.calendar != nil {
		var err error
		midnight, _, err = t.currentSession.calendar.dayBounds(now)
		if err == nil {
			date, err = t.currentSession.calendar.date(midnight)
		}
		if err != nil {
			t.calendarErr = err
			return
		}
	}
	if !t.currentSession.StartTime.Before(midnight) {
		return
	}

	// Write the elapsed slice for the previous day before rebasing, so the two
	// halves are recorded exactly once.
	var slices []*storage.Session
	if t.currentSession.calendar != nil {
		var err error
		slices, err = splitSessionWithCalendar(t.currentSession, midnight, t.currentSession.calendar)
		if err != nil {
			t.calendarErr = err
			return
		}
	} else {
		slices = splitSessionAtMidnight(t.currentSession, midnight)
	}
	t.pendingSessions = append(t.pendingSessions, slices...)

	t.currentSession.StartTime = midnight
	t.currentSession.SliceID = sliceID(t.currentSession.SessionID, midnight)
	t.currentSession.Date = date

}

func (t *Tracker) flushCurrentSession() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closeCurrentSession()
}

func (t *Tracker) flushPendingSessions() {
	t.writeActiveSession(true)
}

func sliceID(sessionID string, start time.Time) string {
	if sessionID == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d", sessionID, start.Unix())
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
	if t.ipcDone != nil {
		defer close(t.ipcDone)
	}
	defer listener.Close()
	slots := make(chan struct{}, 8)
	go func() { <-t.ctx.Done(); listener.Close() }()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if t.ctx.Err() != nil {
				return
			}
			continue
		}
		select {
		case slots <- struct{}{}:
			t.ipcHandlers.Add(1)
			go func() { defer t.ipcHandlers.Done(); defer func() { <-slots }(); t.handleIPCConnection(conn) }()
		default:
			conn.Close()
		}
	}
}

const maxIPCLineBytes = coreapi.MaxIPCBytes

func (t *Tracker) handleIPCConnection(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	request, err := coreapi.ReadRequest(conn)
	if err != nil {
		return
	}
	if !coreapi.TokensMatch(request.Token, t.ipcToken) || t.controller == nil || request.Generation != t.controller.generation {
		return
	}
	response := t.SubmitRequest(request)
	_ = coreapi.WriteResponse(conn, response)
}
