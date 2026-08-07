package email

import (
	"bytes"
	"testing"
	"unicode/utf8"
)

// FuzzExtractBody ensures malformed or hostile MIME input never panics the
// parser and that the extracted body obeys the size limit when parsing
// succeeds.
func FuzzExtractBody(f *testing.F) {
	f.Add([]byte("Content-Type: text/plain\r\n\r\nhello"))
	f.Add([]byte("From: a@example.com\r\nContent-Type: multipart/mixed; boundary=b\r\n\r\n--b\r\nContent-Type: text/plain\r\n\r\nbody\r\n--b--\r\n"))
	f.Add([]byte("Content-Type: text/html\r\n\r\n<p>Hello</p>"))
	f.Add([]byte("not a message"))
	f.Add(bytes.Repeat([]byte("a"), maxBodyBytes+1))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		r := &Reader{}
		body, _, _, _, err := r.extractBody(bytes.NewReader(data))
		if err == nil && len(body) > maxBodyBytes {
			t.Fatalf("extractBody() returned %d-byte body, max is %d", len(body), maxBodyBytes)
		}
		_ = r.createPreview(body, 200)
	})
}

// FuzzStripHTML ensures the HTML-to-text converter never panics on arbitrary
// markup.
func FuzzStripHTML(f *testing.F) {
	f.Add("<p>Hello &amp; welcome</p>")
	f.Add("<script>alert(1)</script><style>.x{}</style><br>")
	f.Add("no tags")
	f.Add("<a href='x'>link</a>")
	f.Fuzz(func(t *testing.T, html string) {
		r := &Reader{}
		_ = r.stripHTML(html)
	})
}

// FuzzDecodeHeader ensures RFC 2047 decoding never panics. Decoded output is
// not asserted to be UTF-8 because legacy charsets (e.g. latin-1) legitimately
// decode to non-UTF-8 bytes.
func FuzzDecodeHeader(f *testing.F) {
	f.Add("=?utf-8?B?0JzQsNGI0LA=?=")
	f.Add("=?UTF-8?Q?Caf=C3=A9?=")
	f.Add("=?iso-8859-1?Q?Keld_J=F8rn_Simonsen?=")
	f.Add("plain header")
	f.Add("=?broken")
	f.Fuzz(func(t *testing.T, s string) {
		_ = decodeHeader(s)
	})
}

// FuzzSafeSlicePrefix ensures the byte-budget truncation never splits a
// multibyte sequence and never emits invalid UTF-8 from a valid UTF-8 input.
// (Invalid UTF-8 inputs are passed through as-is; the function's contract is
// boundary-safe truncation, not input sanitization.)
func FuzzSafeSlicePrefix(f *testing.F) {
	f.Add("你好世界", 6)
	f.Add("abcdef", 3)
	f.Add("é", 1)
	f.Add("", 0)
	f.Fuzz(func(t *testing.T, s string, n int) {
		got := safeSlicePrefix(s, n)
		if utf8.ValidString(s) && !utf8.ValidString(got) {
			t.Fatalf("safeSlicePrefix(%q, %d) returned invalid UTF-8: %q", s, n, got)
		}
	})
}

// FuzzFormatQuotedReply ensures the reply quoting formatter never panics on
// arbitrary inputs.
func FuzzFormatQuotedReply(f *testing.F) {
	f.Add("reply", "original\nbody", "alice@example.com", "2026-08-07 10:00")
	f.Add("", "", "", "")
	f.Add("a\r\nb\r\nc", "x\r\ny", "Name <n@e>", "d")
	f.Fuzz(func(t *testing.T, reply, original, from, date string) {
		_ = FormatQuotedReply(reply, original, from, date)
	})
}
