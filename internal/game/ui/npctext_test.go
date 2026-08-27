package ui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
)

var blue = ui2d.Color{R: 0, G: 0, B: 1, A: 1}

func TestParseNPCText(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    []TextRun
	}{
		{
			name:    "plain text is one run",
			message: "Welcome.",
			want:    []TextRun{{Text: "Welcome.", Color: npcTextColor}},
		},
		{
			name:    "empty",
			message: "",
			want:    nil,
		},
		{
			// This is ref-02: a name in blue inside an otherwise black line.
			name:    "a colored name mid-sentence",
			message: "Welcome. This is ^0000FFClana Nemieri^000000 and we sell guns.",
			want: []TextRun{
				{Text: "Welcome. This is ", Color: npcTextColor},
				{Text: "Clana Nemieri", Color: blue},
				{Text: " and we sell guns.", Color: npcTextColor},
			},
		},
		{
			name:    "a code at the very end leaves nothing behind it",
			message: "Bye.^0000FF",
			want:    []TextRun{{Text: "Bye.", Color: npcTextColor}},
		},
		{
			name:    "a code at the very start",
			message: "^0000FFBlue",
			want:    []TextRun{{Text: "Blue", Color: blue}},
		},
		{
			// Scripts use carets in prose; swallowing them would eat
			// punctuation that was never a color code.
			name:    "a caret that is not a code is text",
			message: "5^2 is 25",
			want:    []TextRun{{Text: "5^2 is 25", Color: npcTextColor}},
		},
		{
			name:    "a truncated code at the end is text",
			message: "oops ^00FF",
			want:    []TextRun{{Text: "oops ^00FF", Color: npcTextColor}},
		},
		{
			name:    "non-hex after the caret is text",
			message: "^ZZZZZZ hello",
			want:    []TextRun{{Text: "^ZZZZZZ hello", Color: npcTextColor}},
		},
		{
			name:    "lower case hex works",
			message: "^0000ffBlue",
			want:    []TextRun{{Text: "Blue", Color: blue}},
		},
		{
			name:    "line breaks survive",
			message: "[Kafra]\nWelcome.",
			want:    []TextRun{{Text: "[Kafra]\nWelcome.", Color: npcTextColor}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseNPCText(tt.message)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseNPCText(%q) =\n  %+v\nwant\n  %+v", tt.message, got, tt.want)
			}
		})
	}
}

// measureByChar stands in for the font: every character is 10 wide, so widths
// in these tests are just lengths.
func measureByChar(s string) float32 { return float32(len(s)) * 10 }

// lineText joins a wrapped line back into a string, for readable assertions.
func lineText(line []TextRun) string {
	var b strings.Builder
	for _, run := range line {
		b.WriteString(run.Text)
	}

	return b.String()
}

func TestWrapNPCText(t *testing.T) {
	tests := []struct {
		name     string
		runs     []TextRun
		maxWidth float32
		want     []string
	}{
		{
			name:     "a short line does not wrap",
			runs:     []TextRun{{Text: "hello there"}},
			maxWidth: 200,
			want:     []string{"hello there"},
		},
		{
			name:     "wrapping happens on spaces",
			runs:     []TextRun{{Text: "aaa bbb ccc"}},
			maxWidth: 80,
			want:     []string{"aaa bbb", "ccc"},
		},
		{
			// The script's own breaks are significant: they format lists.
			name:     "the script's line breaks are kept",
			runs:     []TextRun{{Text: "one\ntwo"}},
			maxWidth: 1000,
			want:     []string{"one", "two"},
		},
		{
			name:     "a blank line between paragraphs survives",
			runs:     []TextRun{{Text: "one\n\ntwo"}},
			maxWidth: 1000,
			want:     []string{"one", "", "two"},
		},
		{
			name:     "a word wider than the box overflows rather than splitting",
			runs:     []TextRun{{Text: "supercalifragilistic"}},
			maxWidth: 50,
			want:     []string{"supercalifragilistic"},
		},
		{
			name: "wrapping carries on across a color change",
			runs: []TextRun{
				{Text: "aaa ", Color: npcTextColor},
				{Text: "bbb ccc", Color: blue},
			},
			maxWidth: 80,
			want:     []string{"aaa bbb", "ccc"},
		},
		{
			name:     "nothing at all is still one empty line",
			runs:     nil,
			maxWidth: 100,
			want:     []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := WrapNPCText(tt.runs, tt.maxWidth, measureByChar)

			got := make([]string, len(lines))
			for i, line := range lines {
				got[i] = lineText(line)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("WrapNPCText() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestWrapKeepsColors pins that wrapping does not lose which run a word came
// from — the whole point of parsing the codes.
func TestWrapKeepsColors(t *testing.T) {
	runs := []TextRun{
		{Text: "black ", Color: npcTextColor},
		{Text: "blue", Color: blue},
	}

	lines := WrapNPCText(runs, 1000, measureByChar)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}

	if len(lines[0]) != 2 {
		t.Fatalf("got %d runs on the line, want 2: %+v", len(lines[0]), lines[0])
	}
	if lines[0][1].Color != blue {
		t.Errorf("second run color = %+v, want blue", lines[0][1].Color)
	}
}

func TestStripNPCMarkup(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"nothing to strip", "Welcome to Prontera.", "Welcome to Prontera."},
		{
			// Straight from the Prontera Guide, which is where this was found.
			name: "a navigation link keeps its label and loses its payload",
			in:   "one for Knights to the <NAVI>[northwest]<INFO>prontera,55,350,0,000,0</INFO></NAVI>",
			want: "one for Knights to the [northwest]",
		},
		{
			name: "two links on one line",
			in:   "<NAVI>[a]<INFO>x,1,1,0,0,0</INFO></NAVI> and <NAVI>[b]<INFO>y,2,2,0,0,0</INFO></NAVI>",
			want: "[a] and [b]",
		},
		{
			name: "lower case tags",
			in:   "<navi>[here]<info>x,1,1,0,0,0</info></navi>",
			want: "[here]",
		},
		{
			name: "an unterminated tag is left alone rather than eating the line",
			in:   "cost < 5 and > 2",
			want: "cost < 5 and > 2",
		},
		{
			name: "an unclosed INFO keeps the text",
			in:   "see <NAVI>[there]<INFO>broken",
			want: "see <NAVI>[there]<INFO>broken",
		},
		{
			name: "line breaks are untouched",
			in:   "[Guide]\n<NAVI>[north]<INFO>prt,1,1,0,0,0</INFO></NAVI>",
			want: "[Guide]\n[north]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripNPCMarkup(tt.in); got != tt.want {
				t.Errorf("StripNPCMarkup(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestParseStripsMarkupBeforeColors pins that the two passes compose: a script
// can color a navigation label, and both have to come out right.
func TestParseStripsMarkupBeforeColors(t *testing.T) {
	runs := ParseNPCText("go ^0000FF<NAVI>[north]<INFO>prt,1,1,0,0,0</INFO></NAVI>^000000 now")

	var text string
	for _, run := range runs {
		text += run.Text
	}

	if text != "go [north] now" {
		t.Errorf("text = %q, want %q", text, "go [north] now")
	}
}
