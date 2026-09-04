package config

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	flagConfig     = flag.String("config", "", "Path to config file")
	flagDebug      = flag.Bool("debug", false, "Enable debug logging")
	flagServer     = flag.String("server", "", "Login server address")
	flagUsername   = flag.String("username", "", "Account name, overriding the config (QA aid)")
	flagPassword   = flag.String("password", "", "Account password, overriding the config (QA aid)")
	flagWindowed   = flag.Bool("windowed", false, "Run in windowed mode")
	flagFullscreen = flag.Bool("fullscreen", false, "Run in fullscreen mode")
	flagWidth      = flag.Int("width", 0, "Window width")
	flagHeight     = flag.Int("height", 0, "Window height")
	flagTrace      = flag.String("trace", "", "Comma-separated trace channels (move, pick, net, render, status, npc, hud, map, cmd, or all)")
	flagShotAfter  = flag.Duration("screenshot-after", 0, "Capture a screenshot this long after startup, then keep running (QA aid)")
	flagShotEvery  = flag.Duration("screenshot-every", 0, "Capture a screenshot on this interval (QA aid)")
	flagAutoLogin  = flag.Bool("autologin", false, "Log in and enter the first character without input (QA aid)")
	flagStopAt     = flag.String("stop-at", "", "Go this far and no further: charselect or charcreate (QA aid)")
	flagMakeChar   = flag.String("make-char", "", "On the creation screen, use this name and press Create (QA aid)")
	flagDebugHUD   = flag.Bool("debug-overlay", false, "Start with the F3 debug overlay open (QA aid)")
	flagNoBGM      = flag.Bool("no-bgm", false, "Run without background music, keeping sound effects")
	flagWalkTo     = flag.String("walk-to", "", "Once in game, walk to this cell, e.g. 156,22 (QA aid)")
	flagMouseAt    = flag.String("mouse-at", "", "Once in game, put the pointer at this window position, e.g. 640,360 (QA aid)")
	flagOpenWindow = flag.String("open-window", "", "Once in game, open these HUD windows, e.g. equip,item (QA aid)")
	flagEquip      = flag.String("equip", "", "Once in game, wear the items in these inventory slots, e.g. 7,8 (QA aid)")
	flagAttack     = flag.Bool("attack-nearest", false, "Once in game, attack the nearest monster (QA aid)")
	flagCast       = flag.String("cast", "", "Once in game, cast these skills by id, e.g. 5,28 (QA aid)")
	flagCastAura   = flag.Bool("cast-aura", false, "Keep the casting ring under the character, to look at it (QA aid)")
	flagRaiseSkill = flag.String("raise-skill", "", "Once in game, spend a skill point on these skills by id, e.g. 19,19 (QA aid)")
	flagItemInfo   = flag.String("item-info", "", "Once in game, open the item information window on this item id (QA aid)")
	flagCardView   = flag.String("card-view", "", "Once in game, open the card drawing window on this card id (QA aid)")

	flagSay sayLines
)

// sayLines collects every --say, in the order they were given.
type sayLines []string

func (s *sayLines) String() string { return strings.Join(*s, " | ") }

func (s *sayLines) Set(v string) error {
	*s = append(*s, v)

	return nil
}

func init() {
	flag.Var(&flagSay, "say",
		"Once in game, type this line into the chat box and send it. Repeatable; "+
			"lines go in order, e.g. --say \"@commands\" --say \"/where\" (QA aid)")
}

// ParseFlags parses command-line flags. Call this early in main().
func ParseFlags() {
	flag.Parse()
}

// Say returns the lines --say asked for, in order.
//
// Every check in this feature is "type something, look at the box", and a
// keyboard is exactly what an unattended run does not have. The lines go
// through the same path typing does, so what is tested is the real one rather
// than a shortcut past the interface.
func Say() []string {
	return flagSay
}

// OpenWindows returns the windows --open-window asked for, by their menu
// button names.
//
// The windows are what most of the interface is, and an unattended capture has
// no hand to press the buttons that open them. Names rather than an enum: the
// flag is read before the interface exists, and the button names are what the
// windows are called everywhere else anyway.
func OpenWindows() []string {
	if *flagOpenWindow == "" {
		return nil
	}

	var names []string
	for _, name := range strings.Split(*flagOpenWindow, ",") {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}

	return names
}

// AttackNearest reports whether --attack-nearest asked for a fight.
//
// Combat is the one thing the other aids cannot reach: it starts with a click
// on a monster, and an unattended run has no hand to click with. Without this
// nothing about a blow — its timing, its animation, what happens when the
// target dies — can be watched without a person sitting there.
func AttackNearest() bool {
	return *flagAttack
}

// CastSkills returns the skill ids --cast asked to be cast, in order.
//
// The other half of what --attack-nearest is for: a skill is cast from a
// hotkey or a window, and an unattended run has neither. Every skill in the
// game answers one of a handful of ways — it goes off, it is refused with a
// reason, or nothing happens at all — and telling those apart is what a
// regression over a whole job's skills needs.
func CastSkills() []int {
	return intList(*flagCast)
}

// RaiseSkills returns the skills --raise-skill asked to spend a point on, in
// order. Repeats are allowed: two of the same id is two points.
func RaiseSkills() []int {
	return intList(*flagRaiseSkill)
}

// ItemInfo returns the item --item-info asked to open the information window
// on, or nought.
func ItemInfo() int {
	ids := intList(*flagItemInfo)
	if len(ids) == 0 {
		return 0
	}

	return ids[0]
}

// CardView returns the card --card-view asked to show the drawing of, or
// nought.
func CardView() int {
	ids := intList(*flagCardView)
	if len(ids) == 0 {
		return 0
	}

	return ids[0]
}

// HoldCastAura reports whether --cast-aura asked for the casting ring to stay.
//
// An effect that lasts as long as a cast is hard to judge in the second or two
// it is on screen, and most casts are shorter than that. This holds it still.
func HoldCastAura() bool {
	return *flagCastAura
}

// EquipSlots returns the inventory slots --equip asked to be worn.
//
// By slot rather than by item id because that is what the wear packet takes,
// and because a capture that has to search the bag first is a capture that
// breaks when the bag changes.
func EquipSlots() []int {
	if *flagEquip == "" {
		return nil
	}

	var slots []int

	for _, field := range strings.Split(*flagEquip, ",") {
		if index, err := strconv.Atoi(strings.TrimSpace(field)); err == nil {
			slots = append(slots, index)
		}
	}

	return slots
}

// intList reads a comma-separated list of numbers, skipping what does not
// parse rather than failing the run — a QA aid that refuses to start over a
// stray comma is worse than one that does what it understood.
func intList(spec string) []int {
	if spec == "" {
		return nil
	}

	var out []int
	for _, field := range strings.Split(spec, ",") {
		if value, err := strconv.Atoi(strings.TrimSpace(field)); err == nil {
			out = append(out, value)
		}
	}

	return out
}

// TraceSpec returns the --trace channel list, empty when tracing is off.
func TraceSpec() string {
	return *flagTrace
}

// ScreenshotAfter returns the one-shot screenshot delay, zero when unset.
func ScreenshotAfter() time.Duration {
	return *flagShotAfter
}

// ScreenshotEvery returns the repeating screenshot interval, zero when unset.
func ScreenshotEvery() time.Duration {
	return *flagShotEvery
}

// AutoLogin reports whether the client should walk the login and character
// select screens by itself.
//
// Getting in game otherwise takes two clicks, which is enough to make every
// check of anything past the login screen need a person at the keyboard. The
// credentials still come from the config, so this grants no access that
// running the client would not.
func AutoLogin() bool {
	return *flagAutoLogin
}

// StopAtCharSelect reports whether the client should hold on the character
// select screen instead of entering the game.
//
// --autologin drives login and character select in one go, and without it the
// client sits at the login window with no way to press its button. So there
// was no way to leave anything on the character select screen long enough to
// look at it, which is where every check of that screen and of character
// creation has to start.
func StopAtCharSelect() bool {
	return *flagStopAt == StopCharSelect
}

// StopAtCharCreate reports whether the client should open character creation
// on the first free slot and hold there.
//
// Every check of that screen otherwise needs a person to double-click a slot,
// which is why its spacing and layout could only be reviewed by hand.
func StopAtCharCreate() bool {
	return *flagStopAt == StopCharCreate
}

// MakeCharName returns the name --make-char asked to create, or "".
//
// Creating a character is the one thing in this feature that cannot be checked
// by looking: it has to reach the server and come back. Driving it from the
// command line is what makes that checkable without a person typing.
func MakeCharName() string {
	return *flagMakeChar
}

// StopAtSpec returns the raw --stop-at value, for reporting one that is not
// a stage we know.
func StopAtSpec() string {
	return *flagStopAt
}

// The stages --stop-at understands.
const (
	// StopCharSelect holds on the character list.
	StopCharSelect = "charselect"
	// StopCharCreate goes one further and opens creation on a free slot.
	StopCharCreate = "charcreate"
)

// DebugOverlay reports whether the F3 overlay should start open.
//
// The overlay is the cheapest proof that a value reached the client, but F3 is
// a keypress, so an unattended --screenshot-after run could never capture it.
// This makes the same readout reachable from a command line.
func DebugOverlay() bool {
	return *flagDebugHUD
}

// NoBGM reports whether background music should be left off.
//
// The music loops for as long as the client runs, which is exactly what you
// do not want while reading a log or listening for a sound effect. Sound
// effects are unaffected.
func NoBGM() bool {
	return *flagNoBGM
}

// WalkTo returns the cell --walk-to asked for, and whether one was given.
//
// Stepping onto a warp is the one thing an unattended run could not do by
// itself: the trigger is a cell, and reaching it takes a click. This issues
// that click once the map is up, through the same path a real one takes, so
// a map change can be captured with nobody at the keyboard.
func WalkTo() (x, y int, ok bool) {
	if *flagWalkTo == "" {
		return 0, 0, false
	}
	if _, err := fmt.Sscanf(*flagWalkTo, "%d,%d", &x, &y); err != nil {
		return 0, 0, false
	}
	return x, y, true
}

// WalkToSpec returns the raw --walk-to value, for reporting one that did not
// parse.
func WalkToSpec() string {
	return *flagWalkTo
}

// MouseAt returns the window position --mouse-at asked for, and whether one
// was given. Which cursor is drawn depends on what the pointer is over, and
// an unattended capture has no hand to put it there.
func MouseAt() (x, y int, ok bool) {
	if *flagMouseAt == "" {
		return 0, 0, false
	}
	if _, err := fmt.Sscanf(*flagMouseAt, "%d,%d", &x, &y); err != nil {
		return 0, 0, false
	}
	return x, y, true
}

// MouseAtSpec returns the raw --mouse-at value, for reporting one that did
// not parse.
func MouseAtSpec() string {
	return *flagMouseAt
}

// ConfigPath returns the explicit config path if provided via --config flag.
func ConfigPath() string {
	return *flagConfig
}

// applyFlags applies CLI flag overrides to the config.
func applyFlags(cfg *Config) {
	if *flagDebug {
		cfg.Logging.Level = "debug"
	}
	if *flagServer != "" {
		cfg.Network.LoginServer = *flagServer
	}
	// Credentials are per-account but the config file is per-checkout, so
	// switching test accounts otherwise means editing config.yaml and putting
	// it back. These override it for one run and are never written to disk.
	// A password in argv is visible to anyone who can read `ps`; these are
	// meant for the throwaway accounts in docs/TEST_ACCOUNTS.md, not real ones.
	if *flagUsername != "" {
		cfg.Network.Username = *flagUsername
	}
	if *flagPassword != "" {
		cfg.Network.Password = *flagPassword
	}
	if *flagWindowed {
		cfg.Graphics.Fullscreen = false
	}
	if *flagFullscreen {
		cfg.Graphics.Fullscreen = true
	}
	if *flagWidth > 0 {
		cfg.Graphics.Width = *flagWidth
	}
	if *flagHeight > 0 {
		cfg.Graphics.Height = *flagHeight
	}
}
