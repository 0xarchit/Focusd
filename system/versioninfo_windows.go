package system

import (
	"syscall"
	"unsafe"
)

// version.dll exposes the resource strings that applications already ship:
// FileDescription and ProductName. Reading them is cheaper than enumerating the
// uninstall registry and works for portable installs too.

var (
	versionDLL                 = syscall.NewLazyDLL("version.dll")
	procGetFileVersionInfoSize = versionDLL.NewProc("GetFileVersionInfoSizeW")
	procGetFileVersionInfo     = versionDLL.NewProc("GetFileVersionInfoW")
	procVerQueryValue          = versionDLL.NewProc("VerQueryValueW")
)

// queryVersionLabel returns FileDescription, falling back to ProductName.
// Values that describe the operating system rather than the application are
// rejected, because explorer.exe reporting "Microsoft Windows" is less useful
// than no label at all.
func queryVersionLabel(exePath string) string {
	if exePath == "" {
		return ""
	}
	pathPtr, err := syscall.UTF16PtrFromString(exePath)
	if err != nil {
		return ""
	}

	var handle uint32
	size, _, _ := procGetFileVersionInfoSize.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&handle)),
	)
	if size == 0 {
		return ""
	}

	buf := make([]byte, size)
	// GetFileVersionInfo returns nonzero on success. It is a BOOL, not an
	// error code, so treating it as GetLastError makes every lookup fail.
	ok, _, _ := procGetFileVersionInfo.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		size,
		uintptr(unsafe.Pointer(&buf[0])),
	)
	if ok == 0 {
		return ""
	}

	for _, key := range []string{
		"\\StringFileInfo\\%04X%04X\\FileDescription",
		"\\StringFileInfo\\%04X%04X\\ProductName",
	} {
		value := queryVersionString(buf, key)
		if value != "" && !isBlocklistedLabel(value) {
			return value
		}
	}
	return ""
}

// queryVersionString resolves a version-resource sub-block. The string table
// lives under a language and codepage pair, so the common English US Unicode
// pair is tried first and then a wildcard scan of the available languages.
func queryVersionString(buf []byte, keyFormat string) string {
	for _, lang := range versionLangs {
		key := expandVersionKey(keyFormat, lang)
		if value := queryVersionBlock(buf, key); value != "" {
			return value
		}
	}
	return ""
}

// versionLangs lists the language and codepage pairs to try, most common first.
var versionLangs = [][2]uint16{
	{0x0409, 0x04B0}, // en-US, Unicode
	{0x0000, 0x04B0}, // neutral, Unicode
	{0x0409, 0x0000}, // en-US, neutral
}

func expandVersionKey(format string, lang [2]uint16) string {
	// The format carries a literal "%04X%04X"; substitute the pair.
	prefix := "\\StringFileInfo\\"
	rest := format[len("\\StringFileInfo\\%04X%04X"):]
	return prefix + langHex(lang[0]) + langHex(lang[1]) + rest
}

func langHex(v uint16) string {
	const hexdigits = "0123456789ABCDEF"
	return string([]byte{
		hexdigits[(v>>12)&0xF],
		hexdigits[(v>>8)&0xF],
		hexdigits[(v>>4)&0xF],
		hexdigits[v&0xF],
	})
}

// versionPtrToUTF16 converts a pointer returned by a Win32 call into a Go
// slice.
//
// go vet's unsafeptr analyzer flags this because the uintptr round-trips
// through the syscall boundary, where the Go collector cannot see the target.
// That is unavoidable for APIs like VerQueryValueW that write a pointer into a
// caller-supplied out-parameter. The memory is the version buffer this package
// already holds, and it is read immediately without escaping.
func versionPtrToUTF16(valuePtr uintptr, maxChars int) []uint16 {
	if valuePtr == 0 {
		return nil
	}
	return unsafe.Slice((*uint16)(unsafe.Pointer(valuePtr)), maxChars)
}

func queryVersionBlock(buf []byte, subBlock string) string {
	blockName, err := syscall.UTF16PtrFromString(subBlock)
	if err != nil {
		return ""
	}

	// VerQueryValueW takes four arguments:
	//   pvsVerInfo, pSubBlock, ppvBuf, puValue
	// puValue reports the resource type, which for a VS_VERSIONINFO string
	// table entry is 31 on modern Windows rather than VT_LPSTR, so it is read
	// but deliberately not filtered on. An empty result is detected by the
	// NUL-terminated scan below.
	var valueType uint32
	var valuePtr uintptr

	ret, _, _ := procVerQueryValue.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(blockName)),
		uintptr(unsafe.Pointer(&valuePtr)),
		uintptr(unsafe.Pointer(&valueType)),
	)
	if ret == 0 || valuePtr == 0 {
		return ""
	}

	// The returned buffer is NUL terminated. The slice is bounded so a
	// malformed resource cannot walk off the end of the mapping, and
	// trimNullTerminated cuts at the NUL.
	const maxChars = 512
	text := versionPtrToUTF16(valuePtr, maxChars)
	if len(text) == 0 {
		return ""
	}
	return trimNullTerminated(text)
}

func trimNullTerminated(text []uint16) string {
	end := 0
	for end < len(text) && text[end] != 0 {
		end++
	}
	return syscall.UTF16ToString(text[:end])
}
