package main

import (
	"bufio"
	"fmt"
	"os"
	"slices"
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
	RESET           = "\x1b[0m"            // Reset all SGR (Select Graphic Rendition) parameters
)

const (
	INPUT_TIMEOUT = 100 * time.Millisecond // Timeout for input operations
	PAGE_STEP     = 20                     // Number of lines to scroll for page up/down
	EXPAND_TABS   = false                  // Whether to expand tabs to spaces
	TAB_SIZE      = 4                      // Number of spaces for a tab if expanded
)

// structs and types

type line struct {
	content string
	render  string
}

type position struct {
	row, col int
}

type FindPositions struct {
	positions []position
	current   int
}

func (f *FindPositions) first() position {
	if len(f.positions) == 0 {
		return position{-1, -1}
	} else {
		return f.positions[0]
	}
}

func (f *FindPositions) next() position {
	if len(f.positions) == 0 {
		return position{-1, -1}
	} else if len(f.positions) == 1 {
		return f.positions[0]
	} else if f.current == len(f.positions)-1 {
		f.current = 0
		return f.positions[f.current]
	} else {
		f.current++
		return f.positions[f.current]
	}
}

func (f *FindPositions) previous() position {
	if len(f.positions) == 0 {
		return position{-1, -1}
	} else if len(f.positions) == 1 {
		return f.positions[0]
	} else if f.current == 0 {
		f.current = len(f.positions) - 1
		return f.positions[f.current]
	} else {
		f.current--
		return f.positions[f.current]
	}
}

type Editor struct {
	reader    *bufio.Reader
	document  *Document
	view      *View
	cursor    *Cursor
	finder    *Finder
	config    *Config
	inputChan chan (KeyEvent)
	buffer    line
}

type View struct {
	cols, rows   int
	maxRowOffset int
	topMargin    int
	botMargin    int
	leftMargin   int
	status       string
}

type Document struct {
	fileName string
	lines    []line
	dirty    bool
}

type Cursor struct {
	row, col                 int
	renderedRow, renderedCol int
	anchor                   int
}

type Finder struct {
	find       bool
	findString string
	matches    FindPositions
}

type KeyEvent struct {
	r   rune
	err error
}

func NewEditor(r *os.File, fileName string) *Editor {
	cfg := LoadConfig()
	return &Editor{
		reader:    bufio.NewReader(r),
		view:      &View{cols: 1, rows: 1, botMargin: 2, status: ""},
		cursor:    &Cursor{1, 1, 1, 1, 1},
		finder:    &Finder{find: false, findString: ""},
		inputChan: make(chan KeyEvent),
		document:  &Document{fileName: fileName, lines: []line{}, dirty: false},
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

func (e *Editor) getWindowSize() (int, int) {
	ncol, nrow, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || (ncol == 0 && nrow == 0) {
		return e.getWindowSizeFallback()
	}
	return ncol, nrow
}

func (e *Editor) getWindowSizeFallback() (int, int) {
	size, err := fmt.Print(BOTTOM_RIGHT)
	if err != nil {
		e.shutdown(fmt.Sprintf("%s", err), 1)
	} else if size != 12 {
		e.shutdown("Window size escape sequence error", 1)
	}
	return e.getCursorPosition()
}

func (e *Editor) getCursorPosition() (int, int) {
	size, err := fmt.Print(CURSOR_POSITION)
	if err != nil {
		e.shutdown(fmt.Sprintf("%s", err), 1)
	} else if size != 4 {
		e.shutdown("cursor position escape sequence error", 1)
	}

	reader := bufio.NewReader(os.Stdin)
	b, err := reader.ReadBytes('R')
	if err != nil {
		e.shutdown(fmt.Sprintf("%s", err), 1)
	} else if b[0] != '\x1b' || b[1] != '[' {
		e.shutdown("cursor position return not valid", 1)
	}
	var nrow, ncol int
	_, err = fmt.Sscanf(fmt.Sprintf("%s", b[1:]), "[%d;%dR", &nrow, &ncol)
	if err != nil {
		e.shutdown(fmt.Sprintf("error parsing: %s", err), 1)
	}
	return nrow, ncol
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

func (e *Editor) calculateRowOffset() {
	for {
		if e.cursor.row-e.view.maxRowOffset < 0 {
			e.view.maxRowOffset--
		} else if e.cursor.row-e.view.maxRowOffset >= e.view.rows-e.view.botMargin {
			e.view.maxRowOffset++
		} else {
			break
		}
	}

}

func (e *Editor) colToRenderCol() int {
	rCol := 0
	l := e.document.lines[e.cursor.row]
	for i := range e.cursor.col {
		if rune(l.content[i]) == TAB {
			rCol += (e.config.TabSize - 1) - (rCol % e.config.TabSize)
		}
		rCol++
	}
	return rCol
}

func (e *Editor) moveCursor(r rune) {
	switch r {
	case ARROW_UP:
		if e.cursor.row > 0 {
			e.cursor.row--
			e.cursor.col = e.cursor.anchor
			if e.cursor.col > len(e.document.lines[e.cursor.row].content) {
				e.cursor.col = len(e.document.lines[e.cursor.row].content)
			}
		}
	case ARROW_DOWN:
		if e.cursor.row < min(len(e.document.lines)-1, e.view.maxRowOffset+e.view.rows-e.view.botMargin) {
			e.cursor.row++
			e.cursor.col = e.cursor.anchor
			if e.cursor.col > len(e.document.lines[e.cursor.row].content) {
				e.cursor.col = len(e.document.lines[e.cursor.row].content)
			}
		}

	case ARROW_LEFT:
		if e.cursor.col > 0 {
			e.cursor.col--
		} else if e.cursor.row > 0 {
			e.cursor.row--
			e.cursor.col = len(e.document.lines[e.cursor.row].content)
		}
		e.cursor.anchor = e.cursor.col
	case ARROW_RIGHT:
		if e.cursor.col < len(e.document.lines[e.cursor.row].content) {
			e.cursor.col++
		} else if e.cursor.row < len(e.document.lines)-e.view.botMargin {
			e.cursor.row++
			e.cursor.col = 0
		}
		e.cursor.anchor = e.cursor.col
	case PAGE_UP:
		for range PAGE_STEP {
			e.moveCursor(ARROW_UP)
		}
	case PAGE_DOWN:
		for range PAGE_STEP {
			e.moveCursor(ARROW_DOWN)
		}
	case HOME:
		e.cursor.col = 0
	case END:
		e.cursor.col = len(e.document.lines[e.cursor.row].content)
	case NEW_LINE:
		e.cursor.row++
		e.cursor.col = 0
	}
}

// rendering

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
	ab := ""
	ab += HIDE_CURSOR
	ab += TOP_LEFT
	ab = e.drawRows(ab)
	ab += fmt.Sprintf(
		"\x1b[%d;%dH",
		e.cursor.renderedRow,
		e.cursor.renderedCol)
	ab += SHOW_CURSOR
	fmt.Print(ab)
}

func (e *Editor) updateState() {
	e.view.cols, e.view.rows = e.getWindowSize()
	e.calculateRowOffset()
	e.cursor.renderedRow = e.cursor.row - e.view.maxRowOffset + e.view.topMargin + 1
	e.cursor.renderedCol = e.colToRenderCol() + e.view.leftMargin + 1
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
			e.finder.matches = FindPositions{}
		case BACKSPACE, DELETE:
			if len(e.finder.findString) > 0 {
				e.finder.findString = e.finder.findString[:len(e.finder.findString)-1]
			}
		case RETURN:
			e.find()
			pos := e.finder.matches.first()
			if pos.row != -1 || pos.col != -1 {
				e.cursor.row = pos.row
				e.cursor.col = pos.col
			}
		case ARROW_DOWN, ARROW_RIGHT:
			pos := e.finder.matches.next()
			if pos.row != -1 || pos.col != -1 {
				e.cursor.row = pos.row
				e.cursor.col = pos.col
			}
		case ARROW_LEFT, ARROW_UP:
			pos := e.finder.matches.previous()
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
			e.saveFile()
			e.document.dirty = false
		case CTRL_X:
			e.cutLine()
		case CTRL_C:
			e.copyLine()
		case CTRL_V:
			e.pasteLine()
		case CTRL_F:
			e.finder.find = true
		case ARROW_UP, ARROW_DOWN, ARROW_RIGHT, ARROW_LEFT, PAGE_UP, PAGE_DOWN, HOME, END:
			e.moveCursor(r)
		case BACKSPACE, DELETE:
			e.deleteRune()
			e.setDirty()
		case RETURN:
			e.insertNewLine()
			e.setDirty()
		case TAB:
			e.handleTab()
			e.setDirty()
		default:
			if unicode.IsPrint(r) {
				e.writeRune(r)
				e.setDirty()
			}
		}
	}

}

func (e *Editor) copyLine() {
	e.buffer = e.document.lines[e.cursor.row]
}

func (e *Editor) cutLine() {
	e.copyLine()
	e.document.lines = slices.Delete(e.document.lines, e.cursor.row, e.cursor.row+1)
	if e.cursor.row >= len(e.document.lines) {
		e.moveCursor(ARROW_UP)
	}
	if len(e.document.lines[e.cursor.row].content) < e.cursor.col {
		e.cursor.col = len(e.document.lines[e.cursor.row].content)
	}
}

func (e *Editor) pasteLine() {
	if e.buffer.content != "" {
		e.document.lines = slices.Insert(e.document.lines, e.cursor.row, e.buffer)
	}
}

func (e *Editor) handleTab() {
	if e.config.ExpandTabs == true {
		for range e.config.TabSize {
			e.writeRune(SPACE)
		}
	} else {
		e.writeRune(TAB)
	}
}

func (e *Editor) writeRune(r rune) {
	row, idx := e.cursor.row, e.cursor.col
	if row < 0 || idx < 0 {
		return
	}

	l := e.document.lines[row]
	if l.content == "" {
		e.document.lines[row].content = fmt.Sprintf("%c", r)
	} else if idx == 0 {
		e.document.lines[row].content = fmt.Sprintf("%c%s", r, l.content)
	} else if idx == len(l.content)+1 {
		e.document.lines[row].content = fmt.Sprintf("%s%c", l.content, r)
	} else {
		e.document.lines[row].content = fmt.Sprintf("%s%c%s", l.content[:idx], r, l.content[idx:])
	}

	e.document.lines[row].render = e.render(e.document.lines[row].content)
	e.moveCursor(ARROW_RIGHT)
}

func (e *Editor) deleteRune() {
	row, idx := e.cursor.row, e.cursor.col
	if row < 0 || idx < 0 || len(e.document.lines) == 0 {
		return
	}
	l := e.document.lines[row]
	if idx == 0 { //case first position
		if row == 0 { // top row do nothing
			return
		}
		e.moveCursor(ARROW_LEFT)            // here because cursor must move first
		if row == len(e.document.lines)-1 { // last row simple operation
			e.document.lines[row-1].content = fmt.Sprintf("%s%s", e.document.lines[row-1].content, l.content)
			e.document.lines = e.document.lines[:row]
		} else { // in the middle need appending
			e.document.lines[row-1].content = fmt.Sprintf("%s%s", e.document.lines[row-1].content, l.content)
			e.document.lines = slices.Delete(e.document.lines, row, row+1) //append(e.document.lines[:row], e.document.lines[row+1:]...)
		}

		row = row - 1 // row to re-render after
	} else { // somewhere in the middle
		e.document.lines[row].content = fmt.Sprintf("%s%s", l.content[:idx-1], l.content[idx:])
		e.moveCursor(ARROW_LEFT)
	}

	e.document.lines[row].render = e.render(e.document.lines[row].content)
}

func (e *Editor) insertNewLine() {
	row, idx := e.cursor.row, e.cursor.col
	if row < 0 || idx < 0 {
		return
	}
	l := e.document.lines[row]
	lineBefore := line{l.content[:idx], e.render(l.content[:idx])}
	lineAfter := line{l.content[idx:], e.render(l.content[idx:])}

	e.document.lines[row] = lineBefore
	e.document.lines = slices.Insert(e.document.lines, row+1, lineAfter)
	e.moveCursor(NEW_LINE)
}

// higher level functionality

func (e *Editor) find() {
	indices := FindPositions{}
	for i, l := range e.document.lines {
		offset := 0
		for {
			idx := strings.Index(l.content[offset:], e.finder.findString)
			if idx == -1 {
				break
			} else {
				absoluteIndex := offset + idx
				indices.positions = append(indices.positions, position{i, absoluteIndex})
				offset = absoluteIndex + len(e.finder.findString)
			}
		}
	}
	e.finder.matches = indices
}

func (e *Editor) Start() (string, int) {
	e.loadFile()
	go e.readKeyPresses()

	for {
		select {
		case res := <-e.inputChan:
			if res.err != nil {
				return fmt.Sprintf("%s", res.err), 1
			} else if res.r == CTRL_Q {
				return fmt.Sprintf("Ctrl-Q"), 0
			} else {
				e.processKeyPress(res.r)
			}
		case <-time.After(INPUT_TIMEOUT):
		}
		e.updateState()
		e.refreshScreen()
	}
}

func (e *Editor) render(s string) string {
	c := ""
	col := 0
	for _, r := range s {
		if r == TAB {
			n := e.config.TabSize - (col % e.config.TabSize)
			c += strings.Repeat(" ", n)
			col += n
		} else {
			c += string(r)
			col++
		}
	}
	return c
}

func (e *Editor) loadFile() error {
	file, err := os.OpenFile(e.document.fileName, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("in loadFile: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		l := scanner.Text()
		e.document.lines = append(e.document.lines, line{l, e.render(l)})
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("in loadFile: %w", err)
	}

	if len(e.document.lines) == 0 {
		e.document.lines = append(e.document.lines, line{"", ""})
	}

	return nil
}

func (e *Editor) saveFile() error {
	file, err := os.OpenFile(e.document.fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("in saveFile: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	s := ""
	for _, l := range e.document.lines {
		s += l.content + "\n"
	}
	_, err = writer.WriteString(s)
	if err != nil {
		return fmt.Errorf("in saveFile: %w", err)
	}
	e.view.status = fmt.Sprintf("%d bytes written to disc", len(s))
	return nil
}
