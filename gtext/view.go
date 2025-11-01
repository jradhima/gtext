package gtext

import (
	"fmt"
	"strings"
)

const (
	LEFT_MARGIN = 6
)

type View struct {
	rows         int
	cols         int
	maxRowOffset int
	topMargin    int
	botMargin    int
	leftMargin   int
	status       string
	footer       *Footer
}

type Footer struct {
	version string
	width   int
	status  string
}

func (f *Footer) render(mode EditorMode, doc *Document, cfg *Config, cursor *Cursor, finder *Finder) string {
	var builder strings.Builder

	welcome := fmt.Sprintf("gtext : v%s", f.version)
	row, col := cursor.ScreenCoords()
	editorState := fmt.Sprintf("[%d:%d] [lines: %d]", row, col, doc.lineCount())

	builder.WriteString(BLACK_ON_WHITE)

	switch mode {
	case EditMode:
		builder.WriteString("Save: Ctrl-S | Exit: Ctrl-Q | Find: Ctrl-F | Cut: Ctrl-X | Copy: Ctrl-C | Paste: Ctrl-V")

	case FindMode:
		builder.WriteString("Exit: Ctrl-F | Search: Return/Enter | Next: Right, Down | Prev: Left, Up | ")
		builder.WriteString(fmt.Sprintf("[find: %s]", finder.findString))

		if finder.numMatches() > 0 {
			builder.WriteString(fmt.Sprintf(" [match: %d/%d]", finder.current+1, finder.numMatches()))
		}
	}

	builder.WriteString(CLEAR_RIGHT + RESET + "\r\n")

	// --- status line ---
	status := f.status
	if status == "" {
		status = doc.fileName
		if doc.dirty {
			status += "*"
		}
	}

	leftPadding := (f.width-len(welcome))/2 - len(editorState)
	rightPadding := (f.width-len(welcome))/2 - len(status)

	builder.WriteString(editorState)
	builder.WriteString(strings.Repeat(" ", max(leftPadding, 0)))
	builder.WriteString(welcome)
	builder.WriteString(strings.Repeat(" ", max(rightPadding, 0)))
	builder.WriteString(status)
	builder.WriteString(CLEAR_RIGHT)

	return builder.String()
}

// updateScrollPosition ensures the cursor is visible in the view
func (e *Editor) updateScrollPosition() {
	row, col := e.cursor.getCoordinates()
	currentLine, err := e.document.getLine(row)
	if err != nil {
		e.shutdown("error fetching current line", 2)
	}

	e.view.calculateRowOffset(row, e.document.lineCount(), e.config.ScrollMargin)
	// e.calculateColOffset()
	e.cursor.renderedRow = row - e.view.maxRowOffset + e.view.topMargin
	e.cursor.renderedCol = e.view.getCursorRenderCol(currentLine, e.config.TabSize, col) + e.view.leftMargin //- e.view.maxColOffset
}

// calculateRowOffset ensures the cursor is visible in the vertical axis
func (v *View) calculateRowOffset(cursorRow, totalLines, scrollMargin int) {
	for {
		cursorScreenY := cursorRow - v.maxRowOffset

		if cursorScreenY < v.topMargin+scrollMargin {
			if v.maxRowOffset > 0 {
				v.maxRowOffset--
			} else {
				break
			}

		} else if cursorScreenY >= v.rows-v.botMargin-scrollMargin {
			maxOffset := totalLines - (v.rows - v.topMargin - v.botMargin) + scrollMargin
			if v.maxRowOffset < maxOffset {
				v.maxRowOffset++
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

// func (v *View) makeLineNumber() {}

// func (v *View) makeLineContent() {}

// func (v *View) makeCommandBar() {}

// func (v *View) makeStatusBar() {}

// func (v *View) makeSearchBar(finder *Finder) string {
// 	text := "Find: Ctrl-F"
// 	bar := BLACK_ON_GREY + text + strings.Repeat(" ", v.cols-len(text)) + RESET + "\r\n"

// 	return bar
// }

func (v *View) drawRows(mode EditorMode, doc *Document, cfg *Config, cursor *Cursor, finder *Finder) string {
	s := ""
	sideLen := v.leftMargin - 1
	for idx := v.maxRowOffset; idx < v.maxRowOffset+v.rows-v.botMargin; idx++ {
		var sideIndex string
		if idx < len(doc.lines) {
			if cfg.ShowLineNumbers {
				sideIndex = fmt.Sprintf("%d", idx+1)
			} else {
				sideIndex = "~"
			}
			s += strings.Repeat(" ", sideLen-len(sideIndex)) + sideIndex + " "
			s += doc.lines[idx].render + CLEAR_RIGHT + "\r\n"
		} else {
			sideIndex = "~" // Ensure sideIndex is always set
			s += strings.Repeat(" ", sideLen-len(sideIndex)) + sideIndex + " " + CLEAR_RIGHT + "\r\n"
		}
	}
	v.footer.status = v.status
	v.footer.width = v.cols
	v.footer.version = "0.0.1"
	s += v.footer.render(mode, doc, cfg, cursor, finder)
	return s
}

func (v *View) refreshScreen(mode EditorMode, doc *Document, cfg *Config, cursor *Cursor, finder *Finder) {
	output := HIDE_CURSOR + TOP_LEFT
	output += v.drawRows(mode, doc, cfg, cursor, finder)
	row, col := cursor.ScreenCoords()
	output += fmt.Sprintf("\x1b[%d;%dH", row, col)
	output += SHOW_CURSOR
	fmt.Print(output)
}
