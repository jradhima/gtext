package gtext

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
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

func (e *Editor) makeFooter() string {
	welcomeString := fmt.Sprintf("gtext -- v%s", VERSION)
	editorState := fmt.Sprintf(
		"[%d:%d] [lines: %d]",
		e.cursor.row+1,
		e.cursor.col+1,
		len(e.document.lines),
	)

	s := BLACK_ON_WHITE
	if e.finder.find {
		findStringDisplay := fmt.Sprintf("[find: %s]", e.finder.findString)
		s += "Exit: Ctrl-F | Search: Return/Enter | Next Match: Right, Down | Previous Match: Left, Up | " + findStringDisplay
		n := len(e.finder.matches.positions)
		if n > 0 {
			s += fmt.Sprintf(" [match: %d/%d]", e.finder.matches.current+1, n)
		}
	} else {
		s += "Save: Ctrl-S | Exit: Ctrl-Q | Find: Ctrl-F | Cut: Ctrl-X | Copy: Ctrl-C | Paste: Ctrl-V"
	}
	s += CLEAR_RIGHT + RESET + "\r\n"

	status := ""
	if e.view.status == "" {
		status = e.document.fileName
		if e.document.dirty {
			status += "*"
		}
	} else {
		status = e.view.status
	}

	leftPadding := (e.view.cols-len(welcomeString))/2 - len(editorState)
	rightPadding := (e.view.cols-len(welcomeString))/2 - len(status)

	s += editorState + strings.Repeat(" ", max(leftPadding, 0)) + welcomeString + strings.Repeat(" ", max(0, rightPadding)) + status + CLEAR_RIGHT
	return s
}

func (e *Editor) drawRows(s string) string {
	maxNumLen := 0
	if e.config.ShowLineNumbers {
		maxNumLen = len(fmt.Sprintf("%d", len(e.document.lines)))
		e.view.leftMargin = maxNumLen + 1
	}

	for idx := e.view.maxRowOffset; idx < e.view.maxRowOffset+e.view.rows-e.view.botMargin; idx++ {
		if idx < len(e.document.lines) {
			if e.config.ShowLineNumbers {
				num := fmt.Sprintf("%d", idx+1)
				s += strings.Repeat(" ", maxNumLen-len(num)) + num + " "
			}

			s += e.document.lines[idx].render + CLEAR_RIGHT + "\r\n"
		} else {
			s += "~" + CLEAR_RIGHT + "\r\n"
		}
	}
	s += e.makeFooter()
	return s
}

func (e *Editor) refreshScreen() {
	row, col := e.cursor.ScreenCoords()

	ab := ""
	ab += HIDE_CURSOR
	ab += TOP_LEFT
	ab = e.drawRows(ab)
	ab += fmt.Sprintf("\x1b[%d;%dH", row, col)
	ab += SHOW_CURSOR
	fmt.Print(ab)
}

func (e *Editor) getWindowSize() (int, int) {
	ncol, nrow, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || (ncol == 0 && nrow == 0) {
		return e.getWindowSizeFallback()
	}
	return ncol, nrow
}

func (e *Editor) getWindowSizeFallback() (int, int) {
	_, err := fmt.Print(BOTTOM_RIGHT)
	if err != nil {
		e.shutdown(fmt.Sprintf("%s", err), 1)
	}
	row, col, err := e.cursor.getPosition()
	if err != nil {
		e.shutdown(fmt.Sprintf("%s", err), 1)
	}
	return row, col
}
