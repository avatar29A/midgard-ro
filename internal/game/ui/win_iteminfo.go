package ui

import (
	"strconv"
	"strings"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/items"
)

// The window that says what an item is.
//
// The inventory shows an icon and a count, and neither says what a thing does
// or who may use it. This is the original's collection window: the item's own
// picture, its name, and then everything the database knows about it — what it
// is, what it hits or stops, what it weighs, and what a character has to be
// before they may wear it.
//
// Opened by item id rather than by inventory slot, so anything that can name
// an item can open one. There are no shops in this client yet; when there are,
// a row in one opens this window with the same call the inventory makes.

// itemInfoWindowID is the frame's id, needed to read its position back and to
// tell a close from a minimize.
const itemInfoWindowID = "hud_item_info"

const (
	itemInfoW float32 = 244
	itemInfoH float32 = 344

	itemInfoPad float32 = 8

	// The picture at the top. The archive keeps a drawing of most items under
	// its collection folder, several times the size of the inventory icon;
	// where there is none the icon is drawn instead, which is small but is
	// the item rather than a blank.
	itemInfoArtW float32 = 92
	itemInfoArtH float32 = 92

	itemInfoTitleScale float32 = 0.8
	itemInfoTextScale  float32 = 0.65
	itemInfoLineH      float32 = 15
)

// itemInfoCollectionPath is where the archive keeps the drawings. Under the
// interface folder, named in the same Korean the icons are.
const itemInfoCollectionPath = skinBasePath + `collection\`

var (
	itemInfoLabel = ui2d.Color{R: 0.35, G: 0.35, B: 0.35, A: 1}
	itemInfoValue = ui2d.Color{R: 0.11, G: 0.11, B: 0.11, A: 1}
	itemInfoName  = ui2d.Color{R: 0.15, G: 0.25, B: 0.55, A: 1}
)

// ShowItemInfo opens the window on an item, or moves it to another one if it
// is already open.
func (b *UI2DBackend) ShowItemInfo(id uint32) {
	if id == 0 {
		return
	}

	b.itemInfoID = id

	// Clearing the closed flag its own X set, or it opens once and never
	// again. The nil check is for a backend built without a context, which
	// the window's own tests do.
	if b.ctx != nil {
		b.ctx.OpenWindow(itemInfoWindowID)
	}
}

// drawItemInfo draws the window, if anything is being looked at.
func (b *UI2DBackend) drawItemInfo(screenW, screenH float32) {
	if b.itemInfoID == 0 {
		return
	}

	openX := (screenW - itemInfoW) / 2
	openY := (screenH - itemInfoH) / 2

	// Closable but not minimizable: a window this size is either wanted or it
	// is not, and a title bar left floating with nothing under it is clutter
	// rather than a convenience.
	if !b.ctx.BeginWindowEx(itemInfoWindowID, openX, openY, itemInfoW, itemInfoH,
		"Item Info", ui2d.WindowOptions{Closable: true}) {
		// Minimized is not closed: the title bar is still drawn and the
		// window is still open, so only a real close puts it away.
		if b.ctx.WindowClosed(itemInfoWindowID) {
			b.itemInfoID = 0
		}

		return
	}

	// Read back after BeginWindow: before, the position is last frame's, and
	// the contents trail the frame while it is dragged.
	x, y := openX, openY
	if rect, ok := b.ctx.WindowRect(itemInfoWindowID); ok {
		x, y = rect.X, rect.Y
	}

	b.drawItemInfoBody(b.itemInfoID, x, y+ui2d.FrameTitleH)

	b.ctx.EndWindow()
}

// drawItemInfoBody draws the picture, the name and the lines under it.
func (b *UI2DBackend) drawItemInfoBody(id uint32, x, bodyY float32) {
	r := b.ctx.Renderer()

	info, known := items.Lookup(id)

	name := info.Name
	if name == "" {
		name = "Item #" + strconv.FormatUint(uint64(id), 10)
	}

	artX := x + itemInfoPad
	artY := bodyY + itemInfoPad

	b.drawItemArt(info.Resource, artX, artY)

	// The name beside the picture rather than over it: names run long, and
	// one centered over a 92-pixel drawing wraps after two words.
	nameX := artX + itemInfoArtW + itemInfoPad
	r.DrawText(nameX, artY+2, name, itemInfoTitleScale, itemInfoName)

	lineY := artY + itemInfoArtH + itemInfoPad

	if !known {
		r.DrawText(artX, lineY, "Nothing known about this item.",
			itemInfoTextScale, itemInfoLabel)

		return
	}

	bottom := bodyY + itemInfoH - ui2d.FrameTitleH - itemInfoPad
	width := itemInfoW - 2*itemInfoPad

	measure := func(text string) float32 {
		w, _ := r.MeasureText(text, itemInfoTextScale)

		return w
	}

	for _, line := range itemInfoLines(id, info) {
		if lineY+itemInfoLineH > bottom {
			break
		}

		r.DrawText(artX, lineY, line.label, itemInfoTextScale, itemInfoLabel)

		// The value runs on from the label and wraps under it. A job list is
		// eleven names long and ran off the right edge of the window, which
		// is where a restriction is least use to anybody.
		valueX := artX + measure(line.label) + 4

		for i, wrapped := range wrapValue(line.value, measure, width-(valueX-artX), width) {
			if lineY+itemInfoLineH > bottom {
				break
			}

			at := valueX
			if i > 0 {
				at = artX
			}

			r.DrawText(at, lineY, wrapped, itemInfoTextScale, itemInfoValue)

			lineY += itemInfoLineH
		}
	}
}

// wrapValue breaks a value across lines: the first as wide as what is left
// beside its label, the rest the whole width under it.
func wrapValue(value string, measure func(string) float32, first, rest float32) []string {
	if value == "" || measure(value) <= first {
		return []string{value}
	}

	var (
		lines []string
		line  string
	)

	width := first

	for _, word := range strings.Split(value, " ") {
		if line == "" {
			line = word

			continue
		}

		if measure(line+" "+word) <= width {
			line += " " + word

			continue
		}

		lines = append(lines, line)
		line = word
		width = rest
	}

	return append(lines, line)
}

// drawItemArt draws the item's own picture, falling back to its icon.
func (b *UI2DBackend) drawItemArt(resource string, x, y float32) {
	r := b.ctx.Renderer()

	// The slot the picture sits in, so an item with no art at all is still a
	// shape rather than a hole in the window.
	r.DrawRect(x, y, itemInfoArtW, itemInfoArtH, itemsCellBg)

	if resource == "" {
		return
	}

	if tex, err := b.texCache.Load(itemInfoCollectionPath + resource + ".bmp"); err == nil {
		r.DrawImage(tex.ID, x, y, itemInfoArtW, itemInfoArtH, ui2d.ColorWhite)

		return
	}

	// No drawing for this one. Most of what has been added in the last decade
	// has none; the icon is small but it is the item.
	if tex, err := b.texCache.Load(itemIconPath + resource + ".bmp"); err == nil {
		const icon = 32

		r.DrawImage(tex.ID,
			x+(itemInfoArtW-icon)/2, y+(itemInfoArtH-icon)/2,
			icon, icon, ui2d.ColorWhite)
	}
}

// itemInfoLine is one row of the window: what the figure is, and the figure.
type itemInfoLine struct {
	label string
	value string
}

// itemInfoLines is everything worth saying about an item, in the order a
// player reads it: what it is, then what it does, then what it costs to carry
// and what it takes to use.
func itemInfoLines(id uint32, info items.Info) []itemInfoLine {
	detail, known := items.DetailOf(id)
	if !known {
		return []itemInfoLine{{label: "Class:", value: string(info.Category)}}
	}

	lines := []itemInfoLine{{label: "Class:", value: itemClassWords(detail)}}

	if detail.Attack > 0 {
		lines = append(lines, itemInfoLine{"Attack:", strconv.Itoa(detail.Attack)})
	}
	if detail.Defense > 0 {
		lines = append(lines, itemInfoLine{"Defense:", strconv.Itoa(detail.Defense)})
	}
	if detail.Range > 0 {
		cells := " cells"
		if detail.Range == 1 {
			cells = " cell"
		}

		lines = append(lines, itemInfoLine{"Range:", strconv.Itoa(detail.Range) + cells})
	}
	if detail.Slots > 0 {
		lines = append(lines, itemInfoLine{"Slots:", strconv.Itoa(detail.Slots)})
	}

	if detail.Weight > 0 {
		lines = append(lines, itemInfoLine{"Weight:", itemWeightWords(detail.Weight)})
	}

	if len(detail.Locations) > 0 {
		lines = append(lines, itemInfoLine{"Worn:", prettyWords(detail.Locations)})
	}

	if detail.Level > 0 {
		lines = append(lines, itemInfoLine{"Item Level:", strconv.Itoa(detail.Level)})
	}
	if detail.MinLevel > 0 {
		lines = append(lines, itemInfoLine{"Required Level:", strconv.Itoa(detail.MinLevel)})
	}

	// Only worth saying of something worn, and only worth saying when it is
	// false: a sword that can be refined is the ordinary case.
	if detail.Worn() && !detail.Refineable {
		lines = append(lines, itemInfoLine{"Refineable:", "no"})
	}

	if detail.Worn() {
		jobs := "All Jobs"
		if len(detail.Jobs) > 0 {
			jobs = prettyWords(detail.Jobs)
		}

		lines = append(lines, itemInfoLine{"Jobs:", jobs})
	}

	if detail.Buy > 0 {
		lines = append(lines, itemInfoLine{"Price:", strconv.Itoa(detail.Buy) + " z"})
	}

	return lines
}

// itemClassWords is what an item is, in as few words as say it.
//
// A weapon's subtype rather than the word "Weapon": a player knows a Dagger is
// a weapon and wants to know it is a dagger, because that is what decides
// whether their character can hold it.
func itemClassWords(detail items.Detail) string {
	if detail.SubType != "" {
		return prettyWord(detail.SubType)
	}

	if detail.Type == "" {
		return "Etc"
	}

	return prettyWord(detail.Type)
}

// itemWeightWords writes the weight the way the character panel counts it.
// The database stores tenths.
func itemWeightWords(tenths int) string {
	whole, rest := tenths/10, tenths%10
	if rest == 0 {
		return strconv.Itoa(whole)
	}

	return strconv.Itoa(whole) + "." + strconv.Itoa(rest)
}

// prettyWords joins a list of the database's own names for a reader.
func prettyWords(words []string) string {
	out := make([]string, 0, len(words))
	for _, word := range words {
		out = append(out, prettyWord(word))
	}

	return strings.Join(out, ", ")
}

// prettyWord turns one of the database's names into words.
//
// Its names are written for a config file — Right_Hand, SuperNovice,
// BardDancer — and printing them as they come puts an underscore in the middle
// of a window and runs two words together.
func prettyWord(word string) string {
	word = strings.ReplaceAll(word, "_", " ")

	var out strings.Builder

	for i, r := range word {
		// A capital after a lower-case letter is where the next word starts.
		if i > 0 && r >= 'A' && r <= 'Z' {
			before := word[i-1]
			if before >= 'a' && before <= 'z' {
				out.WriteByte(' ')
			}
		}

		out.WriteRune(r)
	}

	return out.String()
}
