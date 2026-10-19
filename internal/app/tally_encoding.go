package app

import (
	"bytes"
	"encoding/binary"
	"regexp"
	"unicode/utf16"
	"unicode/utf8"
)

var xmlDeclEncodingRE = regexp.MustCompile(`(?i)(<\?xml[^>]*?encoding\s*=\s*["'])[^"']*(["'])`)

// tallyToUTF8 converts a Tally reply to UTF-8 before it is sanitised.
// Tally answers in UTF-16 LE when the request or its settings ask for
// Unicode; stripping control bytes from that would delete every NUL half of
// each character and leave garbage, so convert first. A UTF-8 BOM is dropped
// too. Plain UTF-8 or ASCII replies are returned unchanged.
func tallyToUTF8(b []byte) []byte {
	var little, ok bool
	switch {
	case bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}):
		return b[3:]
	case bytes.HasPrefix(b, []byte{0xFF, 0xFE}):
		little, ok, b = true, true, b[2:]
	case bytes.HasPrefix(b, []byte{0xFE, 0xFF}):
		little, ok, b = false, true, b[2:]
	case len(b) >= 4 && b[0] == '<' && b[1] == 0 && b[2] != 0 && b[3] == 0:
		little, ok = true, true
	case len(b) >= 4 && b[0] == 0 && b[1] == '<' && b[2] == 0 && b[3] != 0:
		little, ok = false, true
	}
	if !ok {
		return b
	}
	if len(b)%2 == 1 {
		b = b[:len(b)-1]
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		if little {
			u[i] = binary.LittleEndian.Uint16(b[2*i:])
		} else {
			u[i] = binary.BigEndian.Uint16(b[2*i:])
		}
	}
	runes := utf16.Decode(u)
	out := make([]byte, 0, len(runes)+len(runes)/4)
	for _, r := range runes {
		out = utf8.AppendRune(out, r)
	}
	// The declaration would still say UTF-16, which encoding/xml rejects.
	if loc := xmlDeclEncodingRE.FindSubmatchIndex(out); loc != nil && loc[0] < 64 {
		fixed := append([]byte{}, out[:loc[3]]...)
		fixed = append(fixed, "UTF-8"...)
		out = append(fixed, out[loc[4]:]...)
	}
	return out
}
