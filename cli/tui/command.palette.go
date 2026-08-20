package tui

import "unicode/utf8"

// defaultWidth is what we assume when the real terminal width cannot be read.
const defaultWidth = 80

// Item is one row of the palette. It lives here, rather than in the command
// layer, so the reader never needs to know what a command is: it is handed
// labels and hints and draws them.
type Item struct {
	Label string // what gets submitted when the row is chosen, e.g. "/session"
	Hint  string // the short description shown beside it
}

// palette is the list drawn under the input line, and which of its rows is
// highlighted.
type palette struct {
	items    []Item
	selected int
}

// set swaps in a new result set. The selection goes back to the top because the
// list it pointed into is gone: a filter narrowing from three rows to one would
// otherwise leave it pointing past the end.
func (p *palette) set(items []Item) {
	p.items = items
	p.selected = 0
}

func (p *palette) open() bool { return len(p.items) > 0 }

// current is the highlighted row, or false when the palette is closed.
func (p *palette) current() (Item, bool) {
	if !p.open() {
		return Item{}, false
	}

	return p.items[p.selected], true
}

// up and down clamp rather than wrap. In a list of two, wrapping makes it
// impossible to tell whether a keypress registered at all.
func (p *palette) up() {
	if p.selected > 0 {
		p.selected--
	}
}

func (p *palette) down() {
	if p.selected < len(p.items)-1 {
		p.selected++
	}
}

// render returns one finished string per row, each already colored and short
// enough to sit on a single physical line. Keeping rows to one line each is
// what lets the reader move the cursor back up by a count it knows.
func (p *palette) render(width int) []string {
	const gap = "  "

	limit := width - 1
	rows := make([]string, 0, len(p.items))

	for i, item := range p.items {
		selected := i == p.selected

		marker := "  "
		if selected {
			marker = "> "
		}

		head := truncate(marker+item.Label, limit)

		row := head
		if selected {
			row = Cyan(head)
		}

		// The hint gets whatever room is left, and gives way first: a long
		// description should never crowd out the label it describes.
		if room := limit - runes(head) - len(gap); room > 0 && item.Hint != "" {
			row += gap + Gray(truncate(item.Hint, room))
		}

		rows = append(rows, row)
	}

	return rows
}

// truncate keeps a row inside the terminal, counting runes so a multi-byte
// character is never cut in half.
func truncate(text string, limit int) string {
	if limit <= 0 {
		return ""
	}

	if runes(text) <= limit {
		return text
	}

	if limit == 1 {
		return "…"
	}

	return string([]rune(text)[:limit-1]) + "…"
}

// runes is the length that matters for anything cursor-related: cells on
// screen, not bytes in memory.
func runes(text string) int { return utf8.RuneCountInString(text) }
