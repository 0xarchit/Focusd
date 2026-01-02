package storage

type Repository interface {
	InsertSession(s *Session) error
	InsertSessionsBatch(sessions []*Session) error
	UpdateAppDaily(date, appName, exeName string, durationSecs int) error
	GetAppStatsForDate(date string) ([]AppDailyStat, error)
	GetAppUsageTodayMinutes(exeName string) int
	GetAllSessions() ([]Session, error)
	GetSessionsPaginated(limit, offset int, startDate, endDate string) ([]Session, int, error)

	GetAllAppStats() ([]AppDailyStat, error)
	GetAllBrowserStats() ([]AppDailyStat, error)
	UpdateBrowserDaily(date, domainOrTitle string, durationSecs int) error
	GetBrowserStatsForDate(date string) ([]AppDailyStat, error)
	SaveActiveSession(s *ActiveSessionRecord) error
	ClearActiveSession() error
	RecoverActiveSession() (*Session, error)
	GetConfig(key string) (string, error)
	SetConfig(key, value string) error
	IsConsentGranted() bool
	SetConsent(granted bool) error
	IsPaused() bool
	SetPaused(paused bool) error
	EnforceRetention() error
	IsBrowser(exeName string) bool
	Close() error
}

type SQLiteRepository struct{}

func NewSQLiteRepository() *SQLiteRepository {
	return &SQLiteRepository{}
}

func (r *SQLiteRepository) InsertSession(s *Session) error {
	return InsertSession(s)
}

func (r *SQLiteRepository) InsertSessionsBatch(sessions []*Session) error {
	return InsertSessionsBatch(sessions)
}

func (r *SQLiteRepository) UpdateAppDaily(date, appName, exeName string, durationSecs int) error {
	return UpdateAppDaily(date, appName, exeName, durationSecs)
}

func (r *SQLiteRepository) GetAppStatsForDate(date string) ([]AppDailyStat, error) {
	return GetAppStatsForDate(date)
}

func (r *SQLiteRepository) GetAppUsageTodayMinutes(exeName string) int {
	return GetAppUsageTodayMinutes(exeName)
}

func (r *SQLiteRepository) GetAllSessions() ([]Session, error) {
	return GetAllSessions()
}

func (r *SQLiteRepository) GetSessionsPaginated(limit, offset int, startDate, endDate string) ([]Session, int, error) {
	return GetSessionsPaginated(limit, offset, startDate, endDate)
}

func (r *SQLiteRepository) GetAllAppStats() ([]AppDailyStat, error) {
	return GetAllAppStats()
}

func (r *SQLiteRepository) GetAllBrowserStats() ([]AppDailyStat, error) {
	return GetAllBrowserStats()
}

func (r *SQLiteRepository) UpdateBrowserDaily(date, domainOrTitle string, durationSecs int) error {
	return UpdateBrowserDaily(date, domainOrTitle, durationSecs)
}

func (r *SQLiteRepository) GetBrowserStatsForDate(date string) ([]AppDailyStat, error) {
	return GetBrowserStatsForDate(date)
}

func (r *SQLiteRepository) SaveActiveSession(s *ActiveSessionRecord) error {
	return SaveActiveSession(s)
}

func (r *SQLiteRepository) ClearActiveSession() error {
	return ClearActiveSession()
}

func (r *SQLiteRepository) RecoverActiveSession() (*Session, error) {
	return RecoverActiveSession()
}

func (r *SQLiteRepository) GetConfig(key string) (string, error) {
	return GetConfig(key)
}

func (r *SQLiteRepository) SetConfig(key, value string) error {
	return SetConfig(key, value)
}

func (r *SQLiteRepository) IsConsentGranted() bool {
	return IsConsentGranted()
}

func (r *SQLiteRepository) SetConsent(granted bool) error {
	return SetConsent(granted)
}

func (r *SQLiteRepository) IsPaused() bool {
	return IsPaused()
}

func (r *SQLiteRepository) SetPaused(paused bool) error {
	return SetPaused(paused)
}

func (r *SQLiteRepository) EnforceRetention() error {
	return EnforceRetention()
}

func (r *SQLiteRepository) IsBrowser(exeName string) bool {
	return IsBrowser(exeName)
}

func (r *SQLiteRepository) Close() error {
	return Close()
}

var _ Repository = (*SQLiteRepository)(nil)

var defaultRepo Repository

func GetRepository() Repository {
	if defaultRepo == nil {
		defaultRepo = NewSQLiteRepository()
	}
	return defaultRepo
}

func SetRepository(repo Repository) {
	defaultRepo = repo
}
