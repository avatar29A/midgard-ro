// Package command reads what a player typed into the chat box and says what
// kind of thing it is.
//
// RO has two command families and they are not symmetric, which is the whole
// reason this package exists:
//
//   - `@` and `#` are the server's. They are not packets: the client sends the
//     line as ordinary chat and the server pulls the command out before
//     broadcasting it (rAthena clif.cpp, clif_process_message). So routing one
//     means sending it as public chat and nothing more.
//   - `/` is the client's. Nothing on the server parses a leading slash, so a
//     `/` line we do not implement must never be sent — it would be said out
//     loud to everyone in range.
//
// Parsing is kept here, away from anything that can send or draw, so the rules
// above can be tested without a server or a window.
package command

import "strings"

// Sigil is the character a line begins with, which decides where it goes.
type Sigil uint8

const (
	// Speech is an ordinary line, with no command sigil.
	Speech Sigil = iota
	// Slash is a `/` command: ours to answer, never sent as chat.
	Slash
	// At is an `@` command: the server's, carried as ordinary chat.
	At
	// Char is a `#` command: the server's, aimed at another character.
	Char
)

// String names the sigil for traces and errors.
func (s Sigil) String() string {
	switch s {
	case Slash:
		return "slash"
	case At:
		return "at"
	case Char:
		return "char"
	default:
		return "speech"
	}
}

// Line is a parsed input line.
type Line struct {
	// Sigil is which family the line belongs to.
	Sigil Sigil
	// Name is the command word without its sigil, lowercased. Empty for
	// Speech, and empty for a line that is nothing but a bare sigil.
	Name string
	// Args is everything after the command word, with the separating spaces
	// removed from the front and nothing else touched.
	Args string
	// Raw is the line exactly as it was typed. This is what an `@` or `#`
	// command is sent as: the server does its own parsing, and handing it
	// anything reassembled risks differing from what the player wrote.
	Raw string
}

// IsCommand reports whether the line is a command of any family.
func (l Line) IsCommand() bool { return l.Sigil != Speech }

// IsServerCommand reports whether the line is one the server runs — an `@` or
// a `#`. These are sent as chat, and must be sent as public chat: the whisper
// path never checks for commands.
func (l Line) IsServerCommand() bool { return l.Sigil == At || l.Sigil == Char }

// Parse reads one line of input.
//
// A sigil only counts at the very start, so "and/or" is speech and "/who" is a
// command. Leading whitespace is ignored — the chat box trims it already, but
// a line that is only spaces must not become a bare-sigil command.
func Parse(text string) Line {
	raw := text
	trimmed := strings.TrimLeft(text, " \t")
	if trimmed == "" {
		return Line{Sigil: Speech, Raw: raw}
	}

	var sigil Sigil
	switch trimmed[0] {
	case '/':
		sigil = Slash
	case '@':
		sigil = At
	case '#':
		sigil = Char
	default:
		return Line{Sigil: Speech, Raw: raw}
	}

	// Everything after the sigil, split at the first run of spaces.
	body := trimmed[1:]
	name, args := body, ""
	if i := strings.IndexAny(body, " \t"); i >= 0 {
		name = body[:i]
		args = strings.TrimLeft(body[i:], " \t")
	}

	return Line{
		Sigil: sigil,
		// Lowercased because the server matches case-insensitively and a
		// player typing @GO means @go. Args keep their case: character names
		// live there.
		Name: strings.ToLower(name),
		Args: args,
		Raw:  raw,
	}
}
