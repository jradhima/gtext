package gtext

import (
	"fmt"
	"strings"
)

type View struct {
	rows         int
	cols         int
	maxRowOffset int
	topMargin    int
	botMargin    int
	leftMargin   int
	status       string
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

func (v *View) makeFooter(doc *Document, cfg *Config, cursor *Cursor, finder *Finder) string {
	welcomeString := fmt.Sprintf("gtext -- v%s", VERSION)
	editorState := fmt.Sprintf(
		"[%d:%d] [lines: %d]",
		cursor.row+1,
		cursor.col+1,
		doc.lineCount(),
	)

	s := BLACK_ON_WHITE
	if finder.find {
		findStringDisplay := fmt.Sprintf("[find: %s]", finder.findString)
		s += "Exit: Ctrl-F | Search: Return/Enter | Next Match: Right, Down | Previous Match: Left, Up | " + findStringDisplay
		n := len(finder.matches.positions)
		if n > 0 {
			s += fmt.Sprintf(" [match: %d/%d]", finder.matches.current+1, n)
		}
	} else {
		s += "Save: Ctrl-S | Exit: Ctrl-Q | Find: Ctrl-F | Cut: Ctrl-X | Copy: Ctrl-C | Paste: Ctrl-V"
	}
	s += CLEAR_RIGHT + RESET + "\r\n"

	status := ""
	if v.status == "" {
		status = doc.fileName
		if doc.dirty {
			status += "*"
		}
	} else {
		status = v.status
	}

	leftPadding := (v.cols-len(welcomeString))/2 - len(editorState)
	rightPadding := (v.cols-len(welcomeString))/2 - len(status)

	s += editorState + strings.Repeat(" ", max(leftPadding, 0)) + welcomeString + strings.Repeat(" ", max(0, rightPadding)) + status + CLEAR_RIGHT
	return s
}

func (v *View) drawRows(doc *Document, cfg *Config, cursor *Cursor, finder *Finder) string {
	maxNumLen := 0
	s := ""
	if cfg.ShowLineNumbers {
		maxNumLen = len(fmt.Sprintf("%d", doc.lineCount()))
		v.leftMargin = maxNumLen + 1
	}

	for idx := v.maxRowOffset; idx < v.maxRowOffset+v.rows-v.botMargin; idx++ {
		if idx < len(doc.lines) {
			if cfg.ShowLineNumbers {
				num := fmt.Sprintf("%d", idx+1)
				s += strings.Repeat(" ", maxNumLen-len(num)) + num + " "
			}

			s += doc.lines[idx].render + CLEAR_RIGHT + "\r\n"
		} else {
			s += "~" + CLEAR_RIGHT + "\r\n"
		}
	}
	s += v.makeFooter(doc, cfg, cursor, finder)
	return s
}

func (v *View) refreshScreen(doc *Document, cfg *Config, cursor *Cursor, finder *Finder) {
	output := HIDE_CURSOR + TOP_LEFT
	output += v.drawRows(doc, cfg, cursor, finder)
	row, col := cursor.ScreenCoords()
	output += fmt.Sprintf("\x1b[%d;%dH", row, col)
	output += SHOW_CURSOR
	fmt.Print(output)
}
