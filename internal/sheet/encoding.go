package sheet

import (
	"bytes"
	"errors"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

var (
	bomUTF8    = []byte{0xEF, 0xBB, 0xBF}
	bomUTF16LE = []byte{0xFF, 0xFE}
	bomUTF16BE = []byte{0xFE, 0xFF}
)

// utf8BOM is written at the start of every emitted file.
var utf8BOM = bomUTF8

// DetectAndDecode decodes input bytes to a UTF-8 string, detecting a BOM and
// falling back to Windows-1252 for non-UTF-8 input without a BOM.
func DetectAndDecode(b []byte) (string, Encoding, BOM, error) {
	switch {
	case bytes.HasPrefix(b, bomUTF8):
		body := b[len(bomUTF8):]
		if !utf8.Valid(body) {
			return "", 0, 0, errors.New("invalid UTF-8 after BOM")
		}
		return string(body), EncUTF8, BOMUTF8, nil
	case bytes.HasPrefix(b, bomUTF16LE):
		s, err := decodeUTF16(b, unicode.LittleEndian)
		if err != nil {
			return "", 0, 0, err
		}
		return s, EncUTF16LE, BOMUTF16LE, nil
	case bytes.HasPrefix(b, bomUTF16BE):
		s, err := decodeUTF16(b, unicode.BigEndian)
		if err != nil {
			return "", 0, 0, err
		}
		return s, EncUTF16BE, BOMUTF16BE, nil
	}
	if utf8.Valid(b) {
		return string(b), EncUTF8, BOMNone, nil
	}
	out, err := charmap.Windows1252.NewDecoder().Bytes(b)
	if err != nil {
		return "", 0, 0, err
	}
	return string(out), EncWindows1252, BOMNone, nil
}

func decodeUTF16(b []byte, endian unicode.Endianness) (string, error) {
	out, err := unicode.UTF16(endian, unicode.UseBOM).NewDecoder().Bytes(b)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
