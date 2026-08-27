package ui

import (
	"strings"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
)

// colorCodeLen is the length of a `^RRGGBB` sequence.
const colorCodeLen = 7

// npcTextColor is what a message starts in.
//
// Black exactly, not a softened near-black: scripts return to the default with
// `^000000`, and if the default were anything else that would be a visible
// color change rather than a return.
var npcTextColor = ui2d.Color{R: 0, G: 0, B: 0, A: 1}

// TextRun is a piece of text that is all one color.
type TextRun struct {
	Text  string
	Color ui2d.Color
}

// StripNPCMarkup removes the navigation tags scripts embed in their text.
//
// A script writes a place as
// `<NAVI>[northwest]<INFO>prontera,55,350,0,000,0</INFO></NAVI>`. The original
// client turns that into a clickable link showing only the label; the server
// passes it through untouched, so printing the string as it arrives puts the
// raw tags and coordinates on screen.
//
// The label is kept and the payload dropped. Making it actually clickable
// needs minimap navigation, which does not exist yet — but showing
// `[northwest]` is right either way, and showing the coordinates never is.
//
// An unclosed tag is left alone rather than swallowing the rest of the line:
// a stray `<` in prose should cost nothing.
func StripNPCMarkup(message string) string {
	message = removeTagPairs(message, "<info>", "</info>", true)
	message = removeTagPairs(message, "<navi>", "</navi>", false)

	return message
}

// removeTagPairs removes markers, and what is between them when withBody is
// set. Matching is case-insensitive; scripts write these in capitals but
// nothing guarantees it.
func removeTagPairs(message, open, close string, withBody bool) string {
	for {
		lower := strings.ToLower(message)

		start := strings.Index(lower, open)
		if start < 0 {
			return message
		}

		end := strings.Index(lower[start+len(open):], close)
		if end < 0 {
			// Unterminated: leave the whole thing rather than eat the line.
			return message
		}

		end += start + len(open)

		if withBody {
			message = message[:start] + message[end+len(close):]
		} else {
			message = message[:start] + message[start+len(open):end] + message[end+len(close):]
		}
	}
}

// ParseNPCText splits a script's message into colored runs.
//
// Scripts set color inline with `^RRGGBB` and return to black with `^000000`.
// The server passes whatever the script wrote through untouched, so a client
// that does not do this prints `^0000FFClana Nemieri^000000` on screen — which
// is what ref-02 would have looked like.
//
// A caret that is not followed by six hex digits is just a caret: scripts use
// it in ordinary prose, and swallowing it would eat their punctuation.
func ParseNPCText(message string) []TextRun {
	return ParseNPCTextColored(message, npcTextColor)
}

// ParseNPCTextColored is ParseNPCText for text that is not on the dialog's
// parchment, where black is the wrong thing to start in. The chat box passes
// the color the line would have had, so text with no color codes in it keeps
// that color rather than being blackened.
func ParseNPCTextColored(message string, base ui2d.Color) []TextRun {
	message = StripNPCMarkup(message)

	var (
		runs    []TextRun
		current strings.Builder
		color   = base
	)

	flush := func() {
		if current.Len() > 0 {
			runs = append(runs, TextRun{Text: current.String(), Color: color})
			current.Reset()
		}
	}

	for i := 0; i < len(message); {
		if message[i] == '^' && i+colorCodeLen <= len(message) {
			if parsed, ok := parseHexColor(message[i+1 : i+colorCodeLen]); ok {
				flush()
				color = parsed
				i += colorCodeLen

				continue
			}
		}

		current.WriteByte(message[i])
		i++
	}

	flush()

	return runs
}

// parseHexColor reads six hex digits as a color.
func parseHexColor(hex string) (ui2d.Color, bool) {
	if len(hex) != 6 {
		return ui2d.Color{}, false
	}

	var value uint32

	for i := 0; i < len(hex); i++ {
		digit, ok := hexDigit(hex[i])
		if !ok {
			return ui2d.Color{}, false
		}

		value = value<<4 | uint32(digit)
	}

	return ui2d.Color{
		R: float32((value>>16)&0xFF) / 255,
		G: float32((value>>8)&0xFF) / 255,
		B: float32(value&0xFF) / 255,
		A: 1,
	}, true
}

func hexDigit(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}

// WrapNPCText lays runs out into lines no wider than maxWidth.
//
// Line breaks written by the script are kept — they are significant, and
// reflowing the text as one paragraph would mangle every script that formats a
// list. Anything past those is wrapped on spaces. A single word longer than
// the box is left to overflow rather than broken mid-word: it is rare, and a
// word split across lines reads worse than one that runs on.
func WrapNPCText(runs []TextRun, maxWidth float32, measure func(string) float32) [][]TextRun {
	var (
		lines  [][]TextRun
		line   []TextRun
		lineW  float32
		spaceW = measure(" ")
	)

	newline := func() {
		lines = append(lines, line)
		line, lineW = nil, 0
	}

	// append adds a word to the current line, wrapping first if it will not
	// fit and the line already has something on it.
	appendWord := func(word string, color ui2d.Color) {
		width := measure(word)
		gap := float32(0)

		if len(line) > 0 {
			gap = spaceW
		}

		if len(line) > 0 && lineW+gap+width > maxWidth {
			newline()
			gap = 0
		}

		text := word
		if gap > 0 {
			text = " " + word
		}

		// Extend the last run when the color has not changed, so a wrapped
		// sentence is one draw call rather than one per word.
		if n := len(line); n > 0 && line[n-1].Color == color {
			line[n-1].Text += text
		} else {
			line = append(line, TextRun{Text: text, Color: color})
		}

		lineW += gap + width
	}

	for _, run := range runs {
		for i, hard := range strings.Split(run.Text, "\n") {
			if i > 0 {
				newline()
			}

			for _, word := range strings.Fields(hard) {
				appendWord(word, run.Color)
			}
		}
	}

	if len(line) > 0 || len(lines) == 0 {
		lines = append(lines, line)
	}

	return lines
}
