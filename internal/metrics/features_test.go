package metrics_test

import (
	"testing"

	"github.com/petasbytes/go-agent/internal/metrics"
)

func TestCountFeatures_Table(t *testing.T) {
	type exp struct {
		bytes int
		runes int
		words int
		lines int
	}
	cases := []struct {
		name string
		in   string
		exp  exp
	}{
		{
			name: "Empty",
			in:   "",
			exp:  exp{bytes: 0, runes: 0, words: 0, lines: 0},
		},
		{
			name: "ASCII",
			in:   "hello world",
			exp:  exp{bytes: 11, runes: 11, words: 2, lines: 1},
		},
		{
			name: "Multibyte",
			in:   "héllö 世界", // bytes=14, runes=8, words=2, lines=1
			exp:  exp{bytes: 14, runes: 8, words: 2, lines: 1},
		},
		{
			name: "Multiline_NoTrailing",
			in:   "a\nb\ncd", // bytes=6, runes=6, words=3, lines=3
			exp:  exp{bytes: 6, runes: 6, words: 3, lines: 3},
		},
		{
			name: "Multiline_Trailing",
			in:   "a\nb\n", // bytes=4, runes=4, words=2, lines=3
			exp:  exp{bytes: 4, runes: 4, words: 2, lines: 3},
		},
		{
			name: "Whitespace_Tabs_Spaces",
			in:   "  foo\tbar   baz  ", // bytes=17, runes=17, words=3, lines=1
			exp:  exp{bytes: 17, runes: 17, words: 3, lines: 1},
		},
		{
			name: "NBSP",
			in:   "foo\u00A0bar", // bytes=8, runes=7, words=2, lines=1
			exp:  exp{bytes: 8, runes: 7, words: 2, lines: 1},
		},
		{
			name: "OnlyWhitespace",
			in:   " \t\n", // bytes=3, runes=3, words=0, lines=2
			exp:  exp{bytes: 3, runes: 3, words: 0, lines: 2},
		},
		{
			name: "CRLF",
			in:   "a\r\nb\r\nc", // bytes=7, runes=7, words=3, lines=3
			exp:  exp{bytes: 7, runes: 7, words: 3, lines: 3},
		},
		{
			name: "EmSpace",
			in:   "foo\u2003bar", // bytes=9, runes=7, words=2, lines=1
			exp:  exp{bytes: 9, runes: 7, words: 2, lines: 1},
		},
		{
			name: "ZeroWidthSpace_NoSplit",
			in:   "foo\u200Bbar", // bytes=9, runes=7, words=1, lines=1
			exp:  exp{bytes: 9, runes: 7, words: 1, lines: 1},
		},
		{
			name: "Emoji_Astral",
			in:   "👍👍", // bytes=8, runes=2, words=1, lines=1
			exp:  exp{bytes: 8, runes: 2, words: 1, lines: 1},
		},
		{
			name: "Combining_Marks",
			in:   "e\u0301", // "e" + combining acute accent -> 1 glyph, 2 runes, 3 bytes
			exp:  exp{bytes: 3, runes: 2, words: 1, lines: 1},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := metrics.CountFeatures(tc.in)
			if f.Bytes != tc.exp.bytes || f.Runes != tc.exp.runes || f.Words != tc.exp.words || f.Lines != tc.exp.lines {
				t.Fatalf("%s: got %+v, want bytes=%d runes=%d words=%d lines=%d", tc.name, f, tc.exp.bytes, tc.exp.runes, tc.exp.words, tc.exp.lines)
			}
		})
	}
}
