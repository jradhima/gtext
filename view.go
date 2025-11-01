package main

import (
	"fmt"
	"strings"
)

const (
	LEFT_MARGIN = 6
)

type View struct {
	rows, cols   int
	rowOffset    int
	topMargin    int
	bottomMargin int
	leftMargin   int
	status       string
	footer       Footer
	scrollMargin int
}

type Footer struct {
	version string
	width   int
	status  string
}

func NewView(rows, cols int, cfg *Config) *View {
	return &View{
		rows:         rows,
		cols:         cols,
		topMargin:    0,
		bottomMargin: 2,
		leftMargin:   LEFT_MARGIN,
		scrollMargin: cfg.ScrollMargin,
		footer:       Footer{version: VERSION},
	}
}

func (f *Footer) setStatus(msg string) {
	f.status = msg
}

func (f *Footer) clearStatus() {
	f.status = ""
}

// --- Rendering entry point ---
func (v *View) Render(mode EditorMode, doc *Document, cfg *Config, cur *Cursor, finder *Finder) {
	v.footer.width = v.cols
	fmt.Print(HIDE_CURSOR + TOP_LEFT)
	fmt.Print(v.drawContent(mode, doc, cfg, cur, finder))
	row, col := cur.screenCoords()
	fmt.Printf("\x1b[%d;%dH%s", row, col, SHOW_CURSOR)
}

// --- Draw all visible lines and footer ---
func (v *View) drawContent(mode EditorMode, doc *Document, cfg *Config, cur *Cursor, finder *Finder) string {
	var builder strings.Builder
	visibleRows := v.rows - v.bottomMargin

	for screenRow := 0; screenRow < visibleRows; screenRow++ {
		docRow := v.rowOffset + screenRow
		lineText := v.renderLine(doc, docRow, cfg)
		builder.WriteString(lineText)
		builder.WriteString(CLEAR_RIGHT + "\r\n")
	}
	v.footer.width = v.cols
	builder.WriteString(v.footer.render(mode, doc, cfg, cur, finder))
	return builder.String()
}

// --- Draw a single line ---
func (v *View) renderLine(doc *Document, row int, cfg *Config) string {
	sideWidth := v.leftMargin - 1
	if row >= doc.lineCount() {
		return fmt.Sprintf("%s~", strings.Repeat(" ", sideWidth-1))
	}
	lineNum := fmt.Sprintf("%d", row+1)
	if !cfg.ShowLineNumbers {
		lineNum = "~"
	}
	padding := strings.Repeat(" ", sideWidth-len(lineNum))
	return padding + lineNum + " " + doc.lines[row].render
}

func (f *Footer) render(mode EditorMode, doc *Document, cfg *Config, cur *Cursor, finder *Finder) string {
	var builder strings.Builder
	builder.WriteString(BLACK_ON_WHITE)

	switch mode {
	case EditMode:
		builder.WriteString("Save: Ctrl-S | Exit: Ctrl-Q | Find: Ctrl-F | Cut: Ctrl-X | Copy: Ctrl-C | Paste: Ctrl-V")
	case FindMode:
		builder.WriteString("Exit: Ctrl-F | Search: Enter | Next: →↓ | Prev: ←↑ | ")
		builder.WriteString(fmt.Sprintf("[find: %s]", finder.findString))
		if finder.numMatches() > 0 {
			builder.WriteString(fmt.Sprintf(" [match: %d/%d]", finder.current+1, finder.numMatches()))
		}
	}
	builder.WriteString(CLEAR_RIGHT + RESET + "\r\n")

	row, col := cur.coords()
	dirtyMarker := ""
	if doc.dirty {
		dirtyMarker = "*"
	}

	editorState := fmt.Sprintf("[%d:%d] [lines: %d]", row+1, col+1, doc.lineCount())
	center := fmt.Sprintf("gtext v%s", f.version)
	status := doc.fileName + dirtyMarker
	if f.status != "" {
		status = f.status
	}

	// compute padding
	leftPadding := max((f.width-len(center))/2-len(editorState), 0)
	rightPadding := max((f.width-len(center))/2-len(status), 0)

	builder.WriteString(editorState)
	builder.WriteString(strings.Repeat(" ", leftPadding))
	builder.WriteString(center)
	builder.WriteString(strings.Repeat(" ", rightPadding))
	builder.WriteString(status)
	builder.WriteString(CLEAR_RIGHT)

	return builder.String()
}

// --- Scroll logic ---
func (v *View) updateScroll(cursorRow, totalLines int) {
	for {
		screenY := cursorRow - v.rowOffset
		if screenY < v.topMargin+v.scrollMargin {
			if v.rowOffset > 0 {
				v.rowOffset--
			} else {
				break
			}
		} else if screenY >= v.rows-v.bottomMargin-v.scrollMargin {
			maxOffset := (totalLines - 1) - (v.topMargin + v.scrollMargin)
			if v.rowOffset < maxOffset {
				v.rowOffset++
			} else {
				break
			}
		} else {
			break
		}
	}
}

// getCursorRenderCol returns the position of the cursor on the rendered line
func (v *View) getCursorRenderCol(content string, tabSize int, cursorCol int) int {
	rCol := 0
	for i, r := range content {
		if i >= cursorCol {
			break
		}

		if r == TAB {
			rCol += (tabSize - 1) - (rCol % tabSize)
		}
		rCol++
	}

	return rCol
}
