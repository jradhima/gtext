package main

import (
	"bufio"
	"fmt"
	"os"
	"sync"
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

const (
	ErrReturnSeqTerminator = gtextError("unexpected return sequence terminator")
)

type Editor struct {
	reader       *bufio.Reader
	document     *Document
	view         *View
	cursor       *Cursor
	finder       *Finder
	config       *Config
	inputChan    chan KeyEvent
	mode         EditorMode
	commands     *CommandRegistry
	quitChan     chan struct{}
	exiting      bool
	shutdownMsg  string
	exitCode     int
	shutdownOnce sync.Once
}

type EditorMode byte

const (
	EditMode EditorMode = iota
	FindMode
)

type KeyEvent struct {
	r   rune
	err error
}

func NewEditor(r *os.File, fileName string) *Editor {
	cfg := loadConfig()
	e := &Editor{
		reader:      bufio.NewReader(r),
		view:        NewView(1, 1, cfg),
		cursor:      NewCursor(0, 0),
		finder:      &Finder{},
		inputChan:   make(chan KeyEvent, 32),
		document:    NewDocument(fileName, cfg),
		config:      cfg,
		mode:        EditMode,
		commands:    &CommandRegistry{},
		exiting:     false,
		quitChan:    make(chan struct{}),
		shutdownMsg: "",
		exitCode:    0,
	}
	e.registerCommands()
	return e
}

func (e *Editor) registerCommands() {
	e.commands.register(Command{
		key:  CTRL_Q,
		name: "Ctrl-Q",
		desc: "Quit the editor",
		action: func(e *Editor) {
			if e.exiting {
				e.requestShutdown("Quit", 0)
				return
			}

			if e.document.dirty {
				e.view.setStatus("Unsaved changes, press Ctrl-Q again to exit", 0)
				e.exiting = true
				go func() {
					time.Sleep(2 * time.Second)
					e.exiting = false
					e.view.clearStatus()
				}()
				return
			}
			e.requestShutdown("Quit", 0)
		},
	})

	e.commands.register(Command{
		key:  CTRL_S,
		name: "Ctrl-S",
		desc: "Save file to disk",
		action: func(e *Editor) {
			n, err := e.document.SaveToDisk()
			if err != nil {
				e.view.setStatus(fmt.Sprintf("Error saving: %v", err), 2)
			} else {
				e.view.setStatus(fmt.Sprintf("Wrote %d bytes", n), 2)
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
				e.view.setStatus("Find mode", 0)
			case FindMode:
				e.mode = EditMode
				e.finder.reset()
				e.view.setStatus("Edit mode", 0)
			default:
				e.requestShutdown("Unknown editor mode", 1)
			}
		},
	})
}

func (e *Editor) requestShutdown(msg string, code int) {
	e.shutdownOnce.Do(func() {
		e.shutdownMsg = msg
		e.exitCode = code
		close(e.quitChan)
	})
}

func (e *Editor) readInputStream() {
	for {
		r, err := ReadKey(e.reader)
		ke := KeyEvent{r, err}
		select {
		case e.inputChan <- ke:
		case <-e.quitChan:
			return
		}
		if err != nil {
			return
		}
	}
}

func ReadKey(r *bufio.Reader) (rune, error) {
	ch, _, err := r.ReadRune()
	if err != nil {
		return 0, err
	}
	if ch != ESCAPE {
		return ch, nil
	}
	seq, err := r.Peek(2)
	if len(seq) != 2 || seq[0] != byte(CSI) {
		return ESCAPE, nil
	}
	if err != nil {
		return 0, err
	}

	r.ReadRune()
	ch, _, _ = r.ReadRune()
	switch ch {
	case 'A':
		return ARROW_UP, nil
	case 'B':
		return ARROW_DOWN, nil
	case 'C':
		return ARROW_RIGHT, nil
	case 'D':
		return ARROW_LEFT, nil
	case 'H':
		return HOME, nil
	case 'F':
		return END, nil
	case '5':
		ch, _, _ := r.ReadRune()
		if ch == '~' {
			return PAGE_UP, nil
		}
	case '6':
		ch, _, _ := r.ReadRune()
		if ch == '~' {
			return PAGE_DOWN, nil
		}
	case '1', '7':
		ch, _, _ := r.ReadRune()
		if ch == '~' {
			return HOME, nil
		}
	case '4', '8':
		ch, _, _ := r.ReadRune()
		if ch == '~' {
			return END, nil
		}
	default:
		return ESCAPE, nil
	}
	return 0, ErrReturnSeqTerminator
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
		if e.handleError("failed to merge lines", err) {
			return
		}
		e.cursor.moveTo(newRow, newCol)
		e.cursor.anchor = newCol

	} else {
		err := e.document.deleteRune(row, col)
		if e.handleError("failed to delete character", err) {
			return
		}
		e.moveLeft()
	}

}

func (e *Editor) handleNewLine() {
	row, col := e.cursor.coords()

	newRow, newCol, err := e.document.insertNewLine(row, col)

	if e.handleError("failed to insert newline", err) {
		return
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

	if e.handleError("failed to insert character", err) {
		return
	}

	e.moveRight()
}

func (e *Editor) Start() (string, int) {
	e.document.LoadFromDisk()
	go e.readInputStream()

	ticker := time.NewTicker(INPUT_TIMEOUT)
	defer ticker.Stop()

	for {
		select {
		case res := <-e.inputChan:
			if res.err != nil {
				return fmt.Sprintf("%s", res.err), 1
			}
			e.processKeyPress(res.r)
		case <-ticker.C:
		case <-e.quitChan:
			return e.shutdownMsg, e.exitCode
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
		e.requestShutdown("failed to query terminal size (print)", 2)
		return 80, 24
	}
	row, col, err := e.cursor.getPosition()
	if err != nil {
		e.requestShutdown("failed to query cursor position", 2)
		return 80, 24
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
		e.requestShutdown("failed to read current line", 3)
		return
	}

	e.cursor.renderedRow = row - e.view.rowOffset + e.view.topMargin
	e.cursor.renderedCol = e.view.getCursorRenderCol(currentLine, e.config.TabSize, col) + e.view.leftMargin
}

func (e *Editor) updateViewSize() {
	e.view.cols, e.view.rows = e.getWindowSize()
}

func (e *Editor) setDirty() {
	e.document.dirty = true
	e.view.status = ""
}

func (e *Editor) handleError(msg string, err error) bool {
	if err != nil {
		e.view.setStatus(msg, 2)
		return true
	}
	return false
}

func Run(fileName string) int {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting raw terminal mode: %v\n", err)
		return 1
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	editor := NewEditor(os.Stdin, fileName)
	shutdownMessage, exitCode := editor.Start()

	fmt.Print(CLEAR + TOP_LEFT + fmt.Sprintf("Exiting gtext: %s\r\n", shutdownMessage))

	return exitCode
}
