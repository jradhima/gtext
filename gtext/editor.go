package gtext

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

	"golang.org/x/term"
)

// const and setup

const VERSION = "0.0.1"

const (
	// ASCII control characters
	CTRL_F rune = 0x06
	CTRL_Q rune = 0x11
	CTRL_S rune = 0x13
	CTRL_V rune = 0x16
	CTRL_C rune = 0x03
	CTRL_X rune = 0x18

	// Common keyboard characters
	BACKSPACE rune = 0x08
	TAB       rune = 0x09
	RETURN    rune = 0x0d
	ESCAPE    rune = 0x1b
	SPACE     rune = 0x20
	CSI       rune = 0x5b
	DELETE    rune = 0x7f
)

const (
	ARROW_UP    rune = 0xE000
	ARROW_DOWN  rune = 0xE001
	ARROW_RIGHT rune = 0xE002
	ARROW_LEFT  rune = 0xE003
	PAGE_UP     rune = 0xE004
	PAGE_DOWN   rune = 0xE005
	HOME        rune = 0xE006
	END         rune = 0xE007
	NEW_LINE    rune = 0xE008
)

const (
	CLEAR           = "\x1b[2J"            // Clear screen
	CLEAR_RIGHT     = "\x1b[K"             // Clear from cursor to end of line
	TOP_LEFT        = "\x1b[H"             // Move cursor to top-left corner
	BOTTOM_RIGHT    = "\x1b[999C\x1b[999B" // Move cursor to bottom-right (approximated)
	CURSOR_POSITION = "\x1b[6n"            // Request cursor position report
	HIDE_CURSOR     = "\x1b[?25l"          // Hide cursor
	SHOW_CURSOR     = "\x1b[?25h"          // Show cursor
	BLACK_ON_WHITE  = "\x1b[30;47m"        // Set foreground to black, background to white
	BLACK_ON_GREY   = "\x1b[30;48;5;240m"  // Set foreground to black, background to grey
	RESET           = "\x1b[0m"            // Reset all SGR (Select Graphic Rendition) parameters
)

const (
	INPUT_TIMEOUT = 100 * time.Millisecond // Timeout for input operations
	PAGE_STEP     = 20                     // Number of lines to scroll for page up/down
	EXPAND_TABS   = false                  // Whether to expand tabs to spaces
	TAB_SIZE      = 4                      // Number of spaces for a tab if expanded
)

type EditorMode byte

const (
	EditMode EditorMode = iota
	FindMode
)

// structs and types

type Editor struct {
	reader    *bufio.Reader
	document  *Document
	view      *View
	cursor    *Cursor
	finder    *Finder
	config    *Config
	inputChan chan (KeyEvent)
	mode      EditorMode
	// buffer    line
}

type KeyEvent struct {
	r   rune
	err error
}

func NewEditor(r *os.File, fileName string) *Editor {
	cfg := LoadConfig()
	document := NewDocument(fileName, &cfg)
	cursor := NewCursor(0, 0)
	return &Editor{
		reader:    bufio.NewReader(r),
		view:      &View{cols: 1, rows: 1, botMargin: 2, leftMargin: LEFT_MARGIN, footer: &Footer{version: "0.0.1"}},
		cursor:    &cursor,
		finder:    &Finder{find: false, findString: ""},
		inputChan: make(chan KeyEvent),
		document:  &document,
		config:    &cfg,
	}
}

func (e *Editor) shutdown(s string, code int) {
	fmt.Print(CLEAR)
	fmt.Print(TOP_LEFT)

	fmt.Printf("Exiting: %s\r\n", s)
	time.Sleep(250 * time.Millisecond)

	fmt.Print(CLEAR)
	fmt.Print(TOP_LEFT)
	os.Exit(code)
}

// terminal functionality

func (e *Editor) readKeyPresses() {
	for {
		r, _, err := e.reader.ReadRune()
		if err != nil {
			e.inputChan <- KeyEvent{r: ESCAPE, err: err}
			continue
		}

		if r == ESCAPE {
			b, err := e.reader.Peek(1)
			if err != nil || len(b) == 0 {
				e.inputChan <- KeyEvent{r: ESCAPE, err: err}
				continue
			}

			if b[0] == byte(CSI) {
				e.reader.ReadRune()
				b1, _, err := e.reader.ReadRune()
				if err != nil {
					e.inputChan <- KeyEvent{r: ESCAPE, err: err}
					continue
				}
				switch b1 {
				case 'A':
					e.inputChan <- KeyEvent{r: ARROW_UP, err: nil}
				case 'B':
					e.inputChan <- KeyEvent{r: ARROW_DOWN, err: nil}
				case 'C':
					e.inputChan <- KeyEvent{r: ARROW_RIGHT, err: nil}
				case 'D':
					e.inputChan <- KeyEvent{r: ARROW_LEFT, err: nil}
				case '5':
					b2, _, err := e.reader.ReadRune()
					if err != nil {
						e.inputChan <- KeyEvent{r: ESCAPE, err: err}
					}
					if b2 == '~' {
						e.inputChan <- KeyEvent{r: PAGE_UP, err: nil}
					}
				case '6':
					b2, _, err := e.reader.ReadRune()
					if err != nil {
						e.inputChan <- KeyEvent{r: ESCAPE, err: err}
					}
					if b2 == '~' {
						e.inputChan <- KeyEvent{r: PAGE_DOWN, err: nil}
					}
				case '1', '7':
					b2, _, err := e.reader.ReadRune()
					if err != nil {
						e.inputChan <- KeyEvent{r: ESCAPE, err: err}
					}
					if b2 == '~' {
						e.inputChan <- KeyEvent{r: HOME, err: nil}
					}
				case 'H':
					e.inputChan <- KeyEvent{r: HOME, err: err}
				case '4', '8':
					b2, _, err := e.reader.ReadRune()
					if err != nil {
						e.inputChan <- KeyEvent{r: ESCAPE, err: err}
					}
					if b2 == '~' {
						e.inputChan <- KeyEvent{r: END, err: nil}
					}
				case 'F':
					e.inputChan <- KeyEvent{r: END, err: err}

				}
				continue
			}
		}
		e.inputChan <- KeyEvent{r: r, err: err}
	}
}

func (e *Editor) currentLineLength() int {
	currentRow := e.cursor.row
	return e.document.getLineLength(currentRow)
}

func (e *Editor) moveUp() {
	if e.cursor.row > 0 {
		e.cursor.row--

		e.cursor.col = e.cursor.anchor

		targetLength := e.currentLineLength()

		if e.cursor.col > targetLength {
			e.cursor.col = targetLength
		}
	}
}

func (e *Editor) moveDown() {
	maxValidRow := e.document.lineCount() - 1

	if e.cursor.row < maxValidRow {
		e.cursor.row++

		e.cursor.col = e.cursor.anchor

		targetLength := e.currentLineLength()

		if e.cursor.col > targetLength {
			e.cursor.col = targetLength
		}

	}
}

func (e *Editor) moveLeft() {
	if e.cursor.col > 0 {
		e.cursor.col--
	} else if e.cursor.row > 0 {
		e.cursor.row--
		e.cursor.col = e.currentLineLength()
	}

	e.cursor.anchor = e.cursor.col
}

func (e *Editor) moveRight() {
	maxColumn := e.currentLineLength()
	if e.cursor.col < maxColumn {
		e.cursor.col++
	} else if e.cursor.row <= len(e.document.lines)-e.view.botMargin {
		e.cursor.row++
		e.cursor.col = 0
	}

	e.cursor.anchor = e.cursor.col
}

func (e *Editor) setCursorRow(newRow int) {
	if newRow == e.cursor.row {
		return
	}

	e.cursor.row = max(0, newRow)
	e.cursor.col = e.cursor.anchor

	targetLength := e.currentLineLength()
	if e.cursor.col > targetLength {
		e.cursor.col = targetLength
	}
}

func (e *Editor) pageUp() {
	rowsToJump := e.view.rows - e.view.topMargin - e.view.botMargin
	newRow := max(0, e.cursor.row-rowsToJump)
	e.setCursorRow(newRow)
}

func (e *Editor) pageDown() {
	maxValidRow := e.document.lineCount() - 1
	if maxValidRow < 0 {
		return
	}

	rowsToJump := e.view.rows - e.view.topMargin - e.view.botMargin
	newRow := min(maxValidRow, e.cursor.row+rowsToJump)
	e.setCursorRow(newRow)
}

func (e *Editor) moveCursor(r rune) {
	switch r {
	case ARROW_UP:
		e.moveUp()
	case ARROW_DOWN:
		e.moveDown()
	case ARROW_LEFT:
		e.moveLeft()
	case ARROW_RIGHT:
		e.moveRight()
	case PAGE_UP:
		e.pageUp()
	case PAGE_DOWN:
		e.pageDown()
	case HOME:
		e.cursor.col = 0
	case END:
		e.cursor.col = e.currentLineLength()
	}
	e.updateScrollPosition()
}

// rendering

func (e *Editor) updateState() {
	e.view.cols, e.view.rows = e.getWindowSize()
	e.updateScrollPosition()
}

func (e *Editor) setDirty() {
	e.document.dirty = true
	e.view.status = ""
}

// text editing

func (e *Editor) processKeyPress(r rune) {
	switch e.finder.find {
	case true:
		switch r {
		case CTRL_Q:
			e.shutdown("Ctrl+Q", 0)
		case CTRL_F:
			e.finder.find = false
			e.finder.findString = ""
			e.finder.matches = []position{}
		case BACKSPACE, DELETE:
			if len(e.finder.findString) > 0 {
				e.finder.findString = e.finder.findString[:len(e.finder.findString)-1]
			}
		case RETURN:
			e.find()
			pos := e.finder.first()
			if pos.row != -1 || pos.col != -1 {
				e.cursor.row = pos.row
				e.cursor.col = pos.col
			}
		case ARROW_DOWN, ARROW_RIGHT:
			pos := e.finder.next()
			if pos.row != -1 || pos.col != -1 {
				e.cursor.row = pos.row
				e.cursor.col = pos.col
			}
		case ARROW_LEFT, ARROW_UP:
			pos := e.finder.previous()
			if pos.row != -1 || pos.col != -1 {
				e.cursor.row = pos.row
				e.cursor.col = pos.col
			}
		default:
			if unicode.IsPrint(r) || r == TAB {
				e.finder.findString += string(r)
			}
		}
	case false:
		switch r {
		// case CTRL_Q:
		// 	e.shutdown("Ctrl+Q", 0)
		case CTRL_S:
			e.document.SaveToDisk()
			e.document.dirty = false
		// case CTRL_X:
		// 	e.cutLine()
		// case CTRL_C:
		// 	e.copyLine()
		// case CTRL_V:
		// 	e.pasteLine()
		case CTRL_F:
			e.finder.find = true
		case ARROW_UP, ARROW_DOWN, ARROW_RIGHT, ARROW_LEFT, PAGE_UP, PAGE_DOWN, HOME, END:
			e.moveCursor(r)
		case BACKSPACE, DELETE:
			e.handleDelete()
			e.setDirty()
		case RETURN:
			e.handleNewLine()
			e.setDirty()
		case TAB:
			e.handleTab()
			e.setDirty()
		default:
			if unicode.IsPrint(r) || r == SPACE {
				e.handlePrintableRune(r)
				e.setDirty()
			}
		}
	}

}

func (e *Editor) handleDelete() {
	row, col := e.cursor.getCoordinates()

	if col == 0 {
		newRow, newCol, err := e.document.mergeLines(row)
		if err != nil {
			e.shutdown(fmt.Sprintf("Line merge failed: %v", err), 3)
		}
		e.cursor.moveTo(newRow, newCol)
		e.cursor.anchor = newCol

	} else {
		err := e.document.deleteRune(row, col)
		if err != nil {
			e.shutdown(fmt.Sprintf("Delete failed: %v", err), 3)
		}
		e.moveLeft()
	}

}

func (e *Editor) handleNewLine() {
	row, col := e.cursor.getCoordinates()

	newRow, newCol, err := e.document.insertNewLine(row, col)

	if err != nil {
		e.shutdown(fmt.Sprintf("Newline insertion failed: %v", err), 3)
	}

	e.cursor.moveTo(newRow, newCol)
	e.cursor.anchor = newCol
}

// handleTab inserts tabs, according to the editor configuration
// it either inserts a tab rune or expands it as spaces
func (e *Editor) handleTab() {
	row, col := e.cursor.getCoordinates()

	if e.config.ExpandTabs {
		spaces := e.config.TabSize - (col % e.config.TabSize)
		for range spaces {
			e.document.insertRune(row, col, ' ')
			col++
		}
	} else {
		e.document.insertRune(row, col, TAB)
		col++
	}

	e.cursor.moveTo(row, col)
	e.cursor.anchor = col
}

func (e *Editor) handlePrintableRune(r rune) {
	row, col := e.cursor.getCoordinates()

	err := e.document.insertRune(row, col, r)

	if err != nil {
		e.shutdown(fmt.Sprintf("Rune insertion failed: %v", err), 3)
	}

	e.moveRight()
}

// higher level functionality

func (e *Editor) find() {
	indices := []position{}
	for i, l := range e.document.lines {
		offset := 0
		for {
			idx := strings.Index(l.content[offset:], e.finder.findString)
			if idx == -1 {
				break
			} else {
				absoluteIndex := offset + idx
				indices = append(indices, position{i, absoluteIndex})
				offset = absoluteIndex + len(e.finder.findString)
			}
		}
	}
	e.finder.matches = indices
}

func (e *Editor) Start() (string, int) {
	e.document.LoadFromDisk()
	go e.readKeyPresses()

	for {
		select {
		case res := <-e.inputChan:
			if res.err != nil {
				return fmt.Sprintf("%s", res.err), 1
			} else if res.r == CTRL_Q {
				return "Ctrl-Q", 0
			} else {
				e.processKeyPress(res.r)
			}
		case <-time.After(INPUT_TIMEOUT):
		}
		e.updateState()
		e.view.refreshScreen(e.mode, e.document, e.config, e.cursor, e.finder)
	}
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
