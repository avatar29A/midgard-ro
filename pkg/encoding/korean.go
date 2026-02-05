// Package encoding provides text encoding utilities for Ragnarok Online file formats.
package encoding

import (
	"bytes"
	"strings"

	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"
)

// EUCKRToUTF8 converts EUC-KR encoded bytes to UTF-8 string.
// Returns the original string if conversion fails.
func EUCKRToUTF8(data []byte) string {
	// Try EUC-KR decoding
	decoder := korean.EUCKR.NewDecoder()
	result, _, err := transform.Bytes(decoder, data)
	if err != nil {
		// Return as-is if decoding fails
		return string(data)
	}
	return string(result)
}

// EUCKRStringToUTF8 converts an EUC-KR encoded string to UTF-8.
func EUCKRStringToUTF8(s string) string {
	return EUCKRToUTF8([]byte(s))
}

// UTF8ToEUCKR converts UTF-8 string to EUC-KR encoded bytes.
// Returns the original bytes if conversion fails.
func UTF8ToEUCKR(s string) []byte {
	encoder := korean.EUCKR.NewEncoder()
	result, _, err := transform.Bytes(encoder, []byte(s))
	if err != nil {
		return []byte(s)
	}
	return result
}

// NormalizeGRFPath normalizes a GRF file path for case-insensitive lookup.
// RO uses EUC-KR encoded paths, so we need to handle both encodings.
func NormalizeGRFPath(path string) string {
	// Convert backslashes to forward slashes
	path = strings.ReplaceAll(path, "\\", "/")
	// Lowercase for case-insensitive matching
	path = strings.ToLower(path)
	return path
}

// TrimNullBytes removes trailing null bytes from a byte slice.
func TrimNullBytes(data []byte) []byte {
	return bytes.TrimRight(data, "\x00")
}

// TrimNullString removes trailing null bytes and converts to string.
func TrimNullString(data []byte) string {
	return string(TrimNullBytes(data))
}

// FixedStringToUTF8 converts a fixed-size EUC-KR encoded byte array to UTF-8 string.
// Handles null termination and encoding conversion.
func FixedStringToUTF8(data []byte) string {
	// Find null terminator
	nullIdx := bytes.IndexByte(data, 0)
	if nullIdx >= 0 {
		data = data[:nullIdx]
	}
	// Try EUC-KR decoding
	return EUCKRToUTF8(data)
}

// UTF8ToFixedString converts UTF-8 string to a fixed-size EUC-KR encoded byte array.
// Pads with null bytes to fill the specified size.
func UTF8ToFixedString(s string, size int) []byte {
	result := make([]byte, size)
	encoded := UTF8ToEUCKR(s)
	copy(result, encoded)
	return result
}
