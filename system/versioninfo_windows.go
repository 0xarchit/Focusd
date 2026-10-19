//go:build !race

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
	buf := loadVersionBuffer(exePath)
	if buf == nil {
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

// loadVersionBuffer reads the version resource of exePath into memory.
func loadVersionBuffer(exePath string) []byte {
	if exePath == "" {
		return nil
	}
	pathPtr, err := syscall.UTF16PtrFromString(exePath)
	if err != nil {
		return nil
	}

	var handle uint32
	size, _, _ := procGetFileVersionInfoSize.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&handle)),
	)
	if size == 0 {
		return nil
	}

	buf := make([]byte, size)
	// GetFileVersionInfo returns a BOOL, not an error code.
	ok, _, _ := procGetFileVersionInfo.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		size,
		uintptr(unsafe.Pointer(&buf[0])),
	)
	if ok == 0 {
		return nil
	}
	return buf
}

// queryVersionString resolves a version-resource sub-block, trying the language
// and codepage pairs the file declares first, then the fixed fallbacks.
func queryVersionString(buf []byte, keyFormat string) string {
	for _, lang := range append(translationPairs(buf), versionLangs...) {
		key := expandVersionKey(keyFormat, lang)
		if value := queryVersionBlock(buf, key); value != "" {
			return value
		}
	}
	return ""
}

// translationPairs reads the VarFileInfo Translation table: the language and
// codepage pairs the file actually ships. A binary localized only to a language
// outside versionLangs is otherwise unreadable, because its string table is
// keyed by its own pair.
func translationPairs(buf []byte) [][2]uint16 {
	if len(buf) == 0 {
		return nil
	}
	blockName, err := syscall.UTF16PtrFromString(`\VarFileInfo\Translation`)
	if err != nil {
		return nil
	}

	var valueType uint32
	var valuePtr uintptr
	ret, _, _ := procVerQueryValue.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(blockName)),
		uintptr(unsafe.Pointer(&valuePtr)),
		uintptr(unsafe.Pointer(&valueType)),
	)
	if ret == 0 || valuePtr == 0 {
		return nil
	}

	// For Translation the out parameter carries the byte length of an array of
	// uint16 pairs, so every entry is four bytes.
	size := int(valueType)
	if size < 4 || size%4 != 0 {
		return nil
	}
	words := versionPtrToUTF16(valuePtr, buf, size/2)
	pairs := make([][2]uint16, 0, len(words)/2)
	for i := 0; i+1 < len(words); i += 2 {
		pairs = append(pairs, [2]uint16{words[i], words[i+1]})
	}
	return pairs
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

// versionPtrToUTF16 converts a Win32-returned pointer into a bounded Go slice.
// An unbounded read runs past the version buffer when the pointer sits near its
// end, which checkptr reports as fatal under -race.
func versionPtrToUTF16(valuePtr uintptr, base []byte, maxChars int) []uint16 {
	if valuePtr == 0 || len(base) == 0 {
		return nil
	}

	// The pointer must lie inside the buffer we hold; anything else is not ours
	// to read.
	basePtr := uintptr(unsafe.Pointer(&base[0]))
	if valuePtr < basePtr || valuePtr >= basePtr+uintptr(len(base)) {
		return nil
	}

	remaining := (basePtr + uintptr(len(base)) - valuePtr) / 2
	if remaining == 0 {
		return nil
	}
	if maxChars <= 0 || remaining < uintptr(maxChars) {
		maxChars = int(remaining)
	}

	return unsafe.Slice((*uint16)(unsafe.Pointer(valuePtr)), maxChars)
}

func queryVersionBlock(buf []byte, subBlock string) string {
	blockName, err := syscall.UTF16PtrFromString(subBlock)
	if err != nil {
		return ""
	}

	// VerQueryValueW takes four arguments and returns the resource type as 31 for a
	// string table entry, not VT_LPSTR, so valueType is read but not filtered.
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

	const maxChars = 512
	text := versionPtrToUTF16(valuePtr, buf, maxChars)
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
