package main

import (
	"bufio"
	"fmt"
	"os"
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
)

const (
	HIGHLIGHT_MATCH = "\x1b[30;43m"
	BLACK_ON_WHITE  = "\x1b[30;47m"       // Set foreground to black, background to white
	BLACK_ON_GREY   = "\x1b[30;48;5;240m" // Set foreground to black, background to grey
	RESET           = "\x1b[0m"           // Reset all SGR (Select Graphic Rendition) parameters
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
	commands  *CommandRegistry
	exiting   bool
	// buffer    line
}

type KeyEvent struct {
	r   rune
	err error
}

func NewEditor(r *os.File, fileName string) *Editor {
	cfg := loadConfig()
	e := Editor{
		reader:    bufio.NewReader(r),
		view:      NewView(1, 1, cfg),
		cursor:    NewCursor(0, 0),
		finder:    &Finder{},
		inputChan: make(chan KeyEvent),
		document:  NewDocument(fileName, cfg),
		config:    cfg,
		mode:      EditMode,
		commands:  &CommandRegistry{},
		exiting:   false,
	}
	e.registerCommands()
	return &e
}

func (e *Editor) registerCommands() {
	e.commands.register(Command{
		key:  CTRL_Q,
		name: "Ctrl-Q",
		desc: "Quit the editor",
		action: func(e *Editor) {
			if e.exiting {
				e.shutdown("Exiting", 0)
				return
			}

			if e.document.dirty {
				e.view.setStatus("Unsaved changes, press Ctrl-Q again to exit.")
				e.exiting = true
				go func() {
					time.Sleep(2 * time.Second)
					e.exiting = false
					e.view.clearStatus()
				}()
				return
			}
			e.shutdown("Exiting", 0)
		},
	})

	e.commands.register(Command{
		key:  CTRL_S,
		name: "Ctrl-S",
		desc: "Save file to disk",
		action: func(e *Editor) {
			n, err := e.document.SaveToDisk()
			if err != nil {
				e.view.setStatus(fmt.Sprintf("Error saving: %v", err))
			} else {
				e.view.setStatus(fmt.Sprintf("Wrote %d bytes", n))
				e.document.dirty = false
			}
		},
	})

	e.commands.register(Command{
		key:  CTRL_F,
		name: "Ctrl-F",
		desc: "Toggle find mode",
		action: func(e *Editor) {
			switch e.mode {
			case EditMode:
				e.mode = FindMode
				e.view.setStatus("Find mode")
			case FindMode:
				e.mode = EditMode
				e.finder.reset()
				e.view.setStatus("Edit mode")
			default:
				e.shutdown(fmt.Sprintf("Invalid mode: %v", e.mode), 1)
			}
		},
	})

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
	} else if e.cursor.row <= len(e.document.lines)-e.view.bottomMargin {
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
	rowsToJump := e.view.rows - e.view.topMargin - e.view.bottomMargin
	newRow := max(0, e.cursor.row-rowsToJump)
	e.setCursorRow(newRow)
}

func (e *Editor) pageDown() {
	maxValidRow := e.document.lineCount() - 1
	if maxValidRow < 0 {
		return
	}

	rowsToJump := e.view.rows - e.view.topMargin - e.view.bottomMargin
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
}

// rendering

func (e *Editor) updateViewSize() {
	e.view.cols, e.view.rows = e.getWindowSize()
}

func (e *Editor) setDirty() {
	e.document.dirty = true
	e.view.status = ""
}

// text editing

func (e *Editor) processKeyPress(r rune) {
	e.view.clearStatus()
	switch e.mode {
	case EditMode:
		e.handleEditModeKey(r)
	case FindMode:
		e.handleFindModeKey(r)
	}
}

func (e *Editor) handleFindModeKey(r rune) {
	if e.commands.execute(e, r) {
		return
	}

	switch r {
	case RETURN:
		e.handleFind()
	case ARROW_DOWN, ARROW_RIGHT:
		e.findNext()
	case ARROW_LEFT, ARROW_UP:
		e.findPrevious()
	default:
		e.finder.editFindString(r)
	}
}

func (e *Editor) handleFind() {
	e.finder.find(e.document)
	pos := e.finder.first()
	if pos.row != -1 || pos.col != -1 {
		e.cursor.moveTo(pos.row, pos.col)

	}
}

func (e *Editor) findNext() {
	pos := e.finder.next()
	if pos.row != -1 || pos.col != -1 {
		e.cursor.moveTo(pos.row, pos.col)
	}
}

func (e *Editor) findPrevious() {
	pos := e.finder.previous()
	if pos.row != -1 || pos.col != -1 {
		e.cursor.moveTo(pos.row, pos.col)
	}
}

func (e *Editor) handleEditModeKey(r rune) {
	if e.commands.execute(e, r) {
		return
	}

	switch r {
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

func (e *Editor) handleDelete() {
	row, col := e.cursor.coords()

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
	row, col := e.cursor.coords()

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
	row, col := e.cursor.coords()

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
	row, col := e.cursor.coords()

	err := e.document.insertRune(row, col, r)

	if err != nil {
		e.shutdown(fmt.Sprintf("Rune insertion failed: %v", err), 3)
	}

	e.moveRight()
}

// higher level functionality

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
		e.updateViewSize()
		e.updateScroll()
		e.view.Render(e.mode, e.document, e.config, e.cursor, e.finder, e.commands)
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

// updateScroll recalculates the view offset and cursor render position
func (e *Editor) updateScroll() {
	row, col := e.cursor.coords()

	e.view.updateScroll(row, e.document.lineCount())

	// Update cursor rendering positions relative to view
	currentLine, err := e.document.getLine(row)
	if err != nil {
		e.shutdown("failed to get current line", 2)
	}

	e.cursor.renderedRow = row - e.view.rowOffset + e.view.topMargin
	e.cursor.renderedCol = e.view.getCursorRenderCol(currentLine, e.config.TabSize, col) + e.view.leftMargin
}
