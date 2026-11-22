package system

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procQueryInterruptTime                = syscall.NewLazyDLL("api-ms-win-core-realtime-l1-1-0.dll").NewProc("QueryInterruptTime")
	procQueryUnbiasedInterruptTime        = kernel32.NewProc("QueryUnbiasedInterruptTime")
	procQueryBootEnvironment              = syscall.NewLazyDLL("ntdll.dll").NewProc("NtQuerySystemInformation")
	procGetDynamicTimeZoneInformation     = kernel32.NewProc("GetDynamicTimeZoneInformation")
	procSystemTimeToTzSpecificLocalTimeEx = kernel32.NewProc("SystemTimeToTzSpecificLocalTimeEx")
	procTzSpecificLocalTimeToSystemTimeEx = kernel32.NewProc("TzSpecificLocalTimeToSystemTimeEx")
	procSystemTimeToTzSpecificLocalTime   = kernel32.NewProc("SystemTimeToTzSpecificLocalTime")
	procTzSpecificLocalTimeToSystemTime   = kernel32.NewProc("TzSpecificLocalTimeToSystemTime")
)

// ClockSample keeps wall time separate from boot-scoped elapsed clocks (100 ns ticks).
type ClockSample struct {
	WallUTC        time.Time
	InterruptTicks uint64
	UnbiasedTicks  uint64
	BootID         string
	ZoneKey        string
	OffsetSeconds  int
}

type dynamicTimezoneInformation struct {
	Bias                        int32
	StandardName                [32]uint16
	StandardDate                windows.Systemtime
	StandardBias                int32
	DaylightName                [32]uint16
	DaylightDate                windows.Systemtime
	DaylightBias                int32
	TimeZoneKeyName             [128]uint16
	DynamicDaylightTimeDisabled uint8
}

type bootEnvironmentInformation struct {
	BootIdentifier windows.GUID
	FirmwareType   uint32
	Padding        uint32
	BootFlags      uint64
}

// ReadClockSample does not infer boot identity from a mutable civil timestamp.
// BootID is empty if Windows cannot provide its boot identifier.
func ReadClockSample() (ClockSample, error) {
	first, err := readInterruptTicks()
	if err != nil {
		return ClockSample{}, err
	}
	wall := time.Now().UTC()
	unbiased, err := readUnbiasedTicks()
	if err != nil {
		return ClockSample{}, err
	}
	last, err := readInterruptTicks()
	if err != nil {
		return ClockSample{}, err
	}
	if last < first {
		return ClockSample{}, fmt.Errorf("interrupt clock moved backward")
	}
	zone, err := currentTimezone()
	if err != nil {
		return ClockSample{}, err
	}
	local, err := localTimeInZone(wall, &zone)
	if err != nil {
		return ClockSample{}, err
	}
	_, offset := local.Zone()
	bootID, _ := readBootID()
	return ClockSample{
		WallUTC: wall, InterruptTicks: first + (last-first)/2,
		UnbiasedTicks: unbiased, BootID: bootID,
		ZoneKey: syscall.UTF16ToString(zone.TimeZoneKeyName[:]), OffsetSeconds: offset,
	}, nil
}

func readInterruptTicks() (uint64, error) {
	if err := procQueryInterruptTime.Find(); err != nil {
		return 0, fmt.Errorf("interrupt clock unavailable: %w", err)
	}
	var ticks uint64
	procQueryInterruptTime.Call(uintptr(unsafe.Pointer(&ticks)))
	return ticks, nil
}

func readUnbiasedTicks() (uint64, error) {
	if err := procQueryUnbiasedInterruptTime.Find(); err != nil {
		return 0, fmt.Errorf("active elapsed clock unavailable: %w", err)
	}
	var ticks uint64
	result, _, err := procQueryUnbiasedInterruptTime.Call(uintptr(unsafe.Pointer(&ticks)))
	if result == 0 {
		return 0, fmt.Errorf("read active elapsed clock: %w", err)
	}
	return ticks, nil
}

func readBootID() (string, error) {
	if err := procQueryBootEnvironment.Find(); err != nil {
		return "", err
	}
	var info bootEnvironmentInformation
	var returned uint32
	status, _, _ := procQueryBootEnvironment.Call(uintptr(windows.SystemBootEnvironmentInformation), uintptr(unsafe.Pointer(&info)), uintptr(unsafe.Sizeof(info)), uintptr(unsafe.Pointer(&returned)))
	if int32(status) < 0 {
		return "", fmt.Errorf("boot information unavailable: NTSTATUS %#x", uint32(status))
	}
	if returned < uint32(unsafe.Sizeof(info.BootIdentifier)) || returned > uint32(unsafe.Sizeof(info)) || info.BootIdentifier == (windows.GUID{}) {
		return "", fmt.Errorf("invalid boot information length %d", returned)
	}
	return info.BootIdentifier.String(), nil
}

func currentTimezone() (dynamicTimezoneInformation, error) {
	var zone dynamicTimezoneInformation
	result, _, err := procGetDynamicTimeZoneInformation.Call(uintptr(unsafe.Pointer(&zone)))
	if uint32(result) == 0xffffffff {
		return zone, fmt.Errorf("read current timezone: %w", err)
	}
	return zone, nil
}

func windowsTime(t time.Time) windows.Systemtime {
	return windows.Systemtime{Year: uint16(t.Year()), Month: uint16(t.Month()), Day: uint16(t.Day()), Hour: uint16(t.Hour()), Minute: uint16(t.Minute()), Second: uint16(t.Second()), Milliseconds: uint16(t.Nanosecond() / int(time.Millisecond))}
}

func timeFields(t windows.Systemtime, location *time.Location) time.Time {
	return time.Date(int(t.Year), time.Month(t.Month), int(t.Day), int(t.Hour), int(t.Minute), int(t.Second), int(t.Milliseconds)*int(time.Millisecond), location)
}

func localTimeInZone(utc time.Time, zone *dynamicTimezoneInformation) (time.Time, error) {
	if utc.Year() < 1601 || utc.Year() > 9999 {
		return time.Time{}, fmt.Errorf("timestamp year %d is outside Windows timezone conversion range", utc.Year())
	}
	source := windowsTime(utc.UTC())
	var local windows.Systemtime
	var result uintptr
	var err error
	if zone.TimeZoneKeyName[0] == 0 {
		tzi := timezoneRules(zone)
		result, _, err = procSystemTimeToTzSpecificLocalTime.Call(uintptr(unsafe.Pointer(&tzi)), uintptr(unsafe.Pointer(&source)), uintptr(unsafe.Pointer(&local)))
	} else {
		result, _, err = procSystemTimeToTzSpecificLocalTimeEx.Call(uintptr(unsafe.Pointer(zone)), uintptr(unsafe.Pointer(&source)), uintptr(unsafe.Pointer(&local)))
	}
	if result == 0 {
		return time.Time{}, fmt.Errorf("convert current timezone: %w", err)
	}
	wallFields := timeFields(local, time.UTC)
	utcFields := timeFields(source, time.UTC)
	offset := int(wallFields.Sub(utcFields).Seconds())
	name := syscall.UTF16ToString(zone.TimeZoneKeyName[:])
	return utc.In(time.FixedZone(name, offset)), nil
}

// LocalTimeAt refreshes Windows rules instead of relying on Go's cached time.Local.
func LocalTimeAt(utc time.Time) (time.Time, error) {
	zone, err := currentTimezone()
	if err != nil {
		return time.Time{}, err
	}
	return localTimeInZone(utc, &zone)
}

func utcFromLocalFields(local time.Time, zone *dynamicTimezoneInformation) (time.Time, error) {
	if local.Year() < 1601 || local.Year() > 9999 {
		return time.Time{}, fmt.Errorf("calendar year %d is outside Windows timezone conversion range", local.Year())
	}
	source := windowsTime(local)
	var utc windows.Systemtime
	var result uintptr
	var err error
	if zone.TimeZoneKeyName[0] == 0 {
		tzi := timezoneRules(zone)
		result, _, err = procTzSpecificLocalTimeToSystemTime.Call(uintptr(unsafe.Pointer(&tzi)), uintptr(unsafe.Pointer(&source)), uintptr(unsafe.Pointer(&utc)))
	} else {
		result, _, err = procTzSpecificLocalTimeToSystemTimeEx.Call(uintptr(unsafe.Pointer(zone)), uintptr(unsafe.Pointer(&source)), uintptr(unsafe.Pointer(&utc)))
	}
	if result == 0 {
		return time.Time{}, fmt.Errorf("convert local calendar boundary: %w", err)
	}
	return timeFields(utc, time.UTC), nil
}

func timezoneRules(zone *dynamicTimezoneInformation) windows.Timezoneinformation {
	return windows.Timezoneinformation{Bias: zone.Bias, StandardName: zone.StandardName, StandardDate: zone.StandardDate, StandardBias: zone.StandardBias, DaylightName: zone.DaylightName, DaylightDate: zone.DaylightDate, DaylightBias: zone.DaylightBias}
}

// LocalDayBounds returns UTC instants bounding the local calendar day containing at.
// The two midnights are converted independently so DST days need not be 24 hours.
func LocalDayBounds(at time.Time) (time.Time, time.Time, error) {
	zone, err := currentTimezone()
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	local, err := localTimeInZone(at, &zone)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return localDayBoundsInZone(local, &zone)
}

// LocalCalendar retains one observed Windows timezone for an active interval.
// A later OS timezone change must not relabel time already collected.
type LocalCalendar struct{ zone dynamicTimezoneInformation }

func CaptureLocalCalendar() (*LocalCalendar, error) {
	zone, err := currentTimezone()
	if err != nil {
		return nil, err
	}
	return &LocalCalendar{zone: zone}, nil
}

func (c *LocalCalendar) LocalTimeAt(at time.Time) (time.Time, error) {
	return localTimeInZone(at, &c.zone)
}

func (c *LocalCalendar) DayBounds(at time.Time) (time.Time, time.Time, error) {
	local, err := c.LocalTimeAt(at)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return localDayBoundsInZone(local, &c.zone)
}

func localDayBoundsInZone(local time.Time, zone *dynamicTimezoneInformation) (time.Time, time.Time, error) {
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	start, err := utcFromLocalFields(midnight, zone)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err := utcFromLocalFields(midnight.AddDate(0, 0, 1), zone)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if !end.After(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("local calendar day does not advance")
	}
	return start, end, nil
}
