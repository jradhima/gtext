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

const VERSION = "1.0.0"

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
	status       string
	buffer       []string
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
		status:      "Edit Mode",
		buffer:      make([]string, 0),
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
		key:    CTRL_Q,
		name:   "Ctrl-Q",
		desc:   "Quit",
		action: e.handleQuit,
	})

	e.commands.register(Command{
		key:    CTRL_S,
		name:   "Ctrl-S",
		desc:   "Save file",
		action: e.handleSave,
	})

	e.commands.register(Command{
		key:    CTRL_F,
		name:   "Ctrl-F",
		desc:   "Find mode",
		action: e.handleFind,
	})

	e.commands.register(Command{
		key:    CTRL_X,
		name:   "Ctrl-X",
		desc:   "Cut line",
		action: e.handleCut,
	})

	e.commands.register(Command{
		key:    CTRL_C,
		name:   "Ctrl-C",
		desc:   "Copy line",
		action: e.handleCopy,
	})

	e.commands.register(Command{
		key:    CTRL_V,
		name:   "Ctrl-V",
		desc:   "Paste line",
		action: e.handlePaste,
	})
}

func (e *Editor) handleSave() {
	n, err := e.document.SaveToDisk()
	if err != nil {
		e.setStatus(fmt.Sprintf("Error saving: %v", err), 2)
	} else {
		e.setStatus(fmt.Sprintf("Wrote %d bytes", n), 2)
		e.document.dirty = false
	}
}

func (e *Editor) handleCut() {
	currentRow := e.cursor.row
	if currentRow == e.document.lineCount()-1 {
		e.moveUp()
	}
	content, err := e.document.getLine(currentRow)
	if e.handleError("could not copy current line", err) {
		return
	}
	err = e.document.removeLine(currentRow)
	if e.handleError("could not remove current line", err) {
		return
	}
	e.setDirty()
	e.buffer = append(e.buffer, content)
	e.setStatus("cut line", 1)
}

func (e *Editor) handleCopy() {
	currentRow := e.cursor.row
	content, err := e.document.getLine(currentRow)
	if content == "" {
		return
	}
	if e.handleError("could not copy current line", err) {
		return
	}
	e.buffer = append(e.buffer, content)
	e.setStatus("copied line", 1)
}

func (e *Editor) handlePaste() {
	bufferLen := len(e.buffer)
	if bufferLen == 0 {
		return
	}
	currentRow := e.cursor.row
	for idx, content := range e.buffer {
		err := e.document.addLine(currentRow+idx, content)
		if e.handleError("could not insert line", err) {
			return
		}
		e.moveDown()
		e.setDirty()
	}
	e.buffer = make([]string, 0)
	e.setStatus(fmt.Sprintf("pasted %d lines", bufferLen), 1)
}

func (e *Editor) handleFind() {
	switch e.mode {
	case EditMode:
		e.mode = FindMode
		e.setStatus("Find mode", 2)
	case FindMode:
		e.mode = EditMode
		e.setStatus("Edit mode", 2)
		e.finder.reset()
	default:
		e.requestShutdown("Unknown editor mode", 1)
	}
}

func (e *Editor) handleQuit() {
	if e.exiting {
		e.requestShutdown("Quit", 0)
		return
	}

	if e.document.dirty {
		e.setStatus("Unsaved changes, press Ctrl-Q again to exit", 0)
		e.exiting = true
		go func() {
			time.Sleep(2 * time.Second)
			e.exiting = false
			e.clearStatus()
		}()
		return
	}
	e.requestShutdown("Quit", 0)
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

func (e *Editor) currentLineLength() int {
	currentRow := e.cursor.row
	return e.document.getLineLength(currentRow)
}

func (e *Editor) moveUp() {
	e.cursor.setRowTo(e.cursor.row-1, e.document)
}

func (e *Editor) moveDown() {
	e.cursor.setRowTo(e.cursor.row+1, e.document)
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

func (e *Editor) pageUp() {
	rowsToJump := e.view.rows - e.view.topMargin - e.view.bottomMargin
	e.cursor.setRowTo(e.cursor.row-rowsToJump, e.document)
}

func (e *Editor) pageDown() {
	rowsToJump := e.view.rows - e.view.topMargin - e.view.bottomMargin
	e.cursor.setRowTo(e.cursor.row+rowsToJump, e.document)
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
	e.clearStatus()
	switch e.mode {
	case EditMode:
		e.handleEditModeKey(r)
	case FindMode:
		e.handleFindModeKey(r)
	}
}

func (e *Editor) handleFindModeKey(r rune) {
	if e.commands.execute(r) {
		return
	}

	switch r {
	case RETURN:
		e.findMatches()
	case ARROW_DOWN, ARROW_RIGHT:
		e.findNext()
	case ARROW_LEFT, ARROW_UP:
		e.findPrevious()
	default:
		e.finder.editFindString(r)
	}
}

func (e *Editor) findMatches() {
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
	if e.commands.execute(r) {
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

		e.updateComponents()
		e.view.Render(e.mode, e.document, e.config, e.cursor, e.finder, e.commands, len(e.buffer), e.status)
	}
}

// updateComponents recalculates the view offset and cursor render position
func (e *Editor) updateComponents() {
	// 1. View updates its dimensions
	rows, cols, err := getWindowSize()
	if err != nil {
		e.requestShutdown(err.Error(), 1)
	}
	e.view.updateSize(rows, cols) // Assumes View has this method

	e.view.updateScroll(e.cursor.row, e.document.lineCount()) // Assumes View has this

	// 3. Cursor updates its rendered position based on View
	currentLine, err := e.document.getLine(e.cursor.row)
	if err != nil {
		e.requestShutdown("failed to read current line", 3)
		return
	}
	// Assumes Cursor has this method
	e.cursor.updateRenderedPos(e.view, currentLine, e.config.TabSize)
}

func (e *Editor) setDirty() {
	e.document.dirty = true
}

func (e *Editor) handleError(msg string, err error) bool {
	if err != nil {
		e.setStatus(msg, 2)
		return true
	}
	return false
}

func (e *Editor) setStatus(msg string, n int) {
	e.status = msg
	if n > 0 {
		go func() {
			time.Sleep(time.Duration(n) * time.Second)
			e.clearStatus()
		}()
	}
}

func (e *Editor) clearStatus() {
	e.status = ""
}

func Run(fileName string) int {
	fmt.Print("\x1b[?1049h") // switch to alternate screen buffer
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting raw terminal mode: %v\n", err)
		return 1
	}

	editor := NewEditor(os.Stdin, fileName)
	_, exitCode := editor.Start()

	err = term.Restore(int(os.Stdin.Fd()), oldState)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to restore terminal state: %v\n", err)
	}
	fmt.Print("\x1b[?1049l") // switch back to main screen buffer

	return exitCode
}
