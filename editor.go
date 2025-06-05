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
const ESCAPE rune = '\x1b'
const CSI rune = '\x5b'
const CTRL_Q rune = '\x11'
const CTRL_S rune = '\x13'
const CTRL_F rune = '\x06'
const SPACE rune = '\x20'
const BACKSPACE rune = '\x08'
const TAB rune = '\x09'
const DELETE rune = '\x7f'
const RETURN rune = '\x0d'
const CLEAR = "\x1b[2J"
const CLEAR_RIGHT = "\x1b[K"
const TOP_LEFT = "\x1b[H"
const BOTTOM_RIGHT = "\x1b[999C\x1b[999B"
const CURSOR_POSITION = "\x1b[6n"
const HIDE_CURSOR = "\x1b[?25l"
const SHOW_CURSOR = "\x1b[?25h"
const BLACK_ON_WHITE = "\x1b[30;47m"
const RESET = "\x1b[0m"
const INPUT_TIMEOUT = 100 * time.Millisecond
const PAGE_STEP = 20
const EXPAND_TABS = false
const TAB_SIZE = 4

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

// structs and types

type line struct {
	content string
	render  string
}

type position struct {
	row int
	col int
}

type FindPositions struct {
	matches []position
	current int
}

func (f *FindPositions) first() position {
	if len(f.matches) == 0 {
		return position{-1, -1}
	} else {
		return f.matches[0]
	}
}

func (f *FindPositions) next() position {
	if len(f.matches) == 0 {
		return position{-1, -1}
	} else if len(f.matches) == 1 {
		return f.matches[0]
	} else if f.current == len(f.matches)-1 {
		f.current = 0
		return f.matches[f.current]
	} else {
		f.current++
		return f.matches[f.current]
	}
}

func (f *FindPositions) previous() position {
	if len(f.matches) == 0 {
		return position{-1, -1}
	} else if len(f.matches) == 1 {
		return f.matches[0]
	} else if f.current == 0 {
		f.current = len(f.matches) - 1
		return f.matches[f.current]
	} else {
		f.current--
		return f.matches[f.current]
	}
}

type Editor struct {
	reader    *bufio.Reader
	state     EditorState
	inputChan chan (ReadResult)
	lines     []line
}

type EditorState struct {
	numCol       int
	numRow       int
	topMargin    int
	leftMargin   int
	botMargin    int
	showNumbers  bool
	inputTimeout time.Duration
	fileName     string
	row          int
	col          int
	anchor       int
	maxRowOffset int
	expandTabs   bool
	tabSize      int
	renderedCol  int
	renderedRow  int
	dirty        bool
	find         bool
	findString   string
	writeStatus  string
	findMatches  FindPositions
}

type ReadResult struct {
	r   rune
	err error
}

func NewEditorState(showNumbers bool, fileName string) EditorState {
	return EditorState{
		numCol:       1,
		numRow:       1,
		topMargin:    0,
		leftMargin:   0,
		botMargin:    2,
		showNumbers:  showNumbers,
		inputTimeout: INPUT_TIMEOUT,
		fileName:     fileName,
		maxRowOffset: 0,
		expandTabs:   EXPAND_TABS,
		tabSize:      TAB_SIZE,
		renderedCol:  0,
	}
}

func NewEditor(r *os.File, fileName string) *Editor {
	initState := NewEditorState(true, fileName)
	return &Editor{
		reader:    bufio.NewReader(r),
		state:     initState,
		inputChan: make(chan ReadResult),
		lines:     []line{},
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
			e.inputChan <- ReadResult{r: ESCAPE, err: err}
			continue
		}

		if r == ESCAPE {
			b, err := e.reader.Peek(1)
			if err != nil || len(b) == 0 {
				e.inputChan <- ReadResult{r: ESCAPE, err: err}
				continue
			}

			if b[0] == byte(CSI) {
				e.reader.ReadRune()
				b1, _, err := e.reader.ReadRune()
				if err != nil {
					e.inputChan <- ReadResult{r: ESCAPE, err: err}
					continue
				}
				switch b1 {
				case 'A':
					e.inputChan <- ReadResult{r: ARROW_UP, err: nil}
				case 'B':
					e.inputChan <- ReadResult{r: ARROW_DOWN, err: nil}
				case 'C':
					e.inputChan <- ReadResult{r: ARROW_RIGHT, err: nil}
				case 'D':
					e.inputChan <- ReadResult{r: ARROW_LEFT, err: nil}
				case '5':
					b2, _, err := e.reader.ReadRune()
					if err != nil {
						e.inputChan <- ReadResult{r: ESCAPE, err: err}
					}
					if b2 == '~' {
						e.inputChan <- ReadResult{r: PAGE_UP, err: nil}
					}
				case '6':
					b2, _, err := e.reader.ReadRune()
					if err != nil {
						e.inputChan <- ReadResult{r: ESCAPE, err: err}
					}
					if b2 == '~' {
						e.inputChan <- ReadResult{r: PAGE_DOWN, err: nil}
					}
				case '1', '7':
					b2, _, err := e.reader.ReadRune()
					if err != nil {
						e.inputChan <- ReadResult{r: ESCAPE, err: err}
					}
					if b2 == '~' {
						e.inputChan <- ReadResult{r: HOME, err: nil}
					}
				case 'H':
					e.inputChan <- ReadResult{r: HOME, err: err}
				case '4', '8':
					b2, _, err := e.reader.ReadRune()
					if err != nil {
						e.inputChan <- ReadResult{r: ESCAPE, err: err}
					}
					if b2 == '~' {
						e.inputChan <- ReadResult{r: END, err: nil}
					}
				case 'F':
					e.inputChan <- ReadResult{r: END, err: err}

				}
				continue
			}
		}
		e.inputChan <- ReadResult{r: r, err: err}
	}
}

func (e *Editor) calculateRowOffset() {
	for {
		if e.state.row-e.state.maxRowOffset < 0 {
			e.state.maxRowOffset--
		} else if e.state.row-e.state.maxRowOffset >= e.state.numRow-e.state.botMargin {
			e.state.maxRowOffset++
		} else {
			break
		}
	}

}

func (e *Editor) colToRenderCol() int {
	rCol := 0
	l := e.lines[e.state.row]
	for i := range e.state.col {
		if rune(l.content[i]) == TAB {
			rCol += (e.state.tabSize - 1) - (rCol % e.state.tabSize)
		}
		rCol++
	}
	return rCol
}

func (e *Editor) moveCursor(r rune) {
	switch r {
	case ARROW_UP:
		if e.state.row > 0 {
			e.state.row--
			e.state.col = e.state.anchor
			if e.state.col > len(e.lines[e.state.row].content) {
				e.state.col = len(e.lines[e.state.row].content)
			}
		}
	case ARROW_DOWN:
		if e.state.row < len(e.lines)-e.state.botMargin {
			e.state.row++
			e.state.col = e.state.anchor
			if e.state.col > len(e.lines[e.state.row].content) {
				e.state.col = len(e.lines[e.state.row].content)
			}
		}

	case ARROW_LEFT:
		if e.state.col > 0 {
			e.state.col--
		} else if e.state.row > 0 {
			e.state.row--
			e.state.col = len(e.lines[e.state.row].content)
		}
		e.state.anchor = e.state.col
	case ARROW_RIGHT:
		if e.state.col < len(e.lines[e.state.row].content) {
			e.state.col++
		} else if e.state.row < len(e.lines)-e.state.botMargin {
			e.state.row++
			e.state.col = 0
		}
		e.state.anchor = e.state.col
	case PAGE_UP:
		for range PAGE_STEP {
			e.moveCursor(ARROW_UP)
		}
	case PAGE_DOWN:
		for range PAGE_STEP {
			e.moveCursor(ARROW_DOWN)
		}
	case HOME:
		e.state.col = 0
	case END:
		e.state.col = len(e.lines[e.state.row].content)
	case NEW_LINE:
		e.state.row++
		e.state.col = 0
	}
}

// rendering

func (e *Editor) makeFooter() string {
	welcomeString := fmt.Sprintf("gtext -- v%s", VERSION)
	editorState := fmt.Sprintf(
		"[%d:%d] [lines: %d]",
		e.state.row+1,
		e.state.col+1,
		len(e.lines),
	)

	s := BLACK_ON_WHITE
	if e.state.find {
		findStringDisplay := fmt.Sprintf("[find: %s]", e.state.findString)
		s += "Exit: Ctrl-F | Search: Return/Enter | Next Match: Right, Down | Previous Match: Left, Up | " + findStringDisplay
		n := len(e.state.findMatches.matches)
		if n > 0 {
			s += fmt.Sprintf(" [match: %d/%d]", e.state.findMatches.current+1, n)
		}
	} else {
		s += "Save: Ctrl-S | Exit: Ctrl-Q | Find: Ctrl-F"
	}
	s += CLEAR_RIGHT + RESET + "\r\n"

	writeStatus := ""
	if e.state.writeStatus == "" {
		writeStatus = e.state.fileName
		if e.state.dirty {
			writeStatus += "*"
		}
	} else {
		writeStatus = e.state.writeStatus
	}

	leftPadding := (e.state.numCol-len(welcomeString))/2 - len(editorState)
	rightPadding := (e.state.numCol-len(welcomeString))/2 - len(writeStatus)

	s += editorState + strings.Repeat(" ", max(leftPadding, 0)) + welcomeString + strings.Repeat(" ", max(0, rightPadding)) + writeStatus + CLEAR_RIGHT
	return s
}

func (e *Editor) drawRows(s string) string {
	maxNumLen := 0
	if e.state.showNumbers {
		maxNumLen = len(fmt.Sprintf("%d", len(e.lines)-e.state.botMargin))
		e.state.leftMargin = maxNumLen + 1
	}

	for idx := e.state.maxRowOffset; idx < e.state.maxRowOffset+e.state.numRow-e.state.botMargin; idx++ {
		if idx < len(e.lines) {
			if e.state.showNumbers {
				num := fmt.Sprintf("%d", idx+1)
				s += strings.Repeat(" ", maxNumLen-len(num)) + num + " "
			}

			s += e.lines[idx].render + CLEAR_RIGHT + "\r\n"
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
		e.state.renderedRow,
		e.state.renderedCol)
	ab += SHOW_CURSOR
	fmt.Print(ab)
}

func (e *Editor) updateState() {
	e.state.numCol, e.state.numRow = e.getWindowSize()
	e.calculateRowOffset()
	e.state.renderedRow = e.state.row - e.state.maxRowOffset + e.state.topMargin + 1
	e.state.renderedCol = e.colToRenderCol() + e.state.leftMargin + 1
}

func (e *Editor) setDirty() {
	e.state.dirty = true
	e.state.writeStatus = ""
}

// text editing

func (e *Editor) processKeyPress(r rune) {
	switch e.state.find {
	case true:
		switch r {
		case CTRL_Q:
			e.shutdown("Ctrl+Q", 0)
		case CTRL_F:
			e.state.find = false
			e.state.findString = ""
			e.state.findMatches = FindPositions{}
		case BACKSPACE, DELETE:
			if len(e.state.findString) > 0 {
				e.state.findString = e.state.findString[:len(e.state.findString)-1]
			}
		case RETURN:
			e.find()
			pos := e.state.findMatches.first()
			if pos.row != -1 || pos.col != -1 {
				e.state.row = pos.row
				e.state.col = pos.col
			}
		case ARROW_DOWN, ARROW_RIGHT:
			pos := e.state.findMatches.next()
			if pos.row != -1 || pos.col != -1 {
				e.state.row = pos.row
				e.state.col = pos.col
			}
		case ARROW_LEFT, ARROW_UP:
			pos := e.state.findMatches.previous()
			if pos.row != -1 || pos.col != -1 {
				e.state.row = pos.row
				e.state.col = pos.col
			}
		default:
			if unicode.IsPrint(r) || r == TAB {
				e.state.findString += string(r)
			}
		}
	case false:
		switch r {
		// case CTRL_Q:
		// 	e.shutdown("Ctrl+Q", 0)
		case CTRL_S:
			e.saveFile()
			e.state.dirty = false
		case CTRL_F:
			e.state.find = true
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

func (e *Editor) handleTab() {
	if e.state.expandTabs == true {
		for range e.state.tabSize {
			e.writeRune(SPACE)
		}
	} else {
		e.writeRune(TAB)
	}
}

func (e *Editor) writeRune(r rune) {
	row, idx := e.state.row, e.state.col
	if row < 0 || idx < 0 {
		return
	}

	l := e.lines[row]
	if l.content == "" {
		e.lines[row].content = fmt.Sprintf("%c", r)
	} else if idx == 0 {
		e.lines[row].content = fmt.Sprintf("%c%s", r, l.content)
	} else if idx == len(l.content)+1 {
		e.lines[row].content = fmt.Sprintf("%s%c", l.content, r)
	} else {
		e.lines[row].content = fmt.Sprintf("%s%c%s", l.content[:idx], r, l.content[idx:])
	}

	e.lines[row].render = e.render(e.lines[row].content)
	e.moveCursor(ARROW_RIGHT)
}

func (e *Editor) deleteRune() {
	row, idx := e.state.row, e.state.col
	if row < 0 || idx < 0 || len(e.lines) == 0 {
		return
	}
	l := e.lines[row]
	if idx == 0 { //case first position
		if row == 0 { // top row do nothing
			return
		}
		e.moveCursor(ARROW_LEFT)   // here because cursor must move first
		if row == len(e.lines)-1 { // last row simple operation
			e.lines[row-1].content = fmt.Sprintf("%s%s", e.lines[row-1].content, l.content)
			e.lines = e.lines[:row]
		} else { // in the middle need appending
			e.lines[row-1].content = fmt.Sprintf("%s%s", e.lines[row-1].content, l.content)
			e.lines = slices.Delete(e.lines, row, row+1) //append(e.lines[:row], e.lines[row+1:]...)
		}

		row = row - 1 // row to re-render after
	} else { // somewhere in the middle
		e.lines[row].content = fmt.Sprintf("%s%s", l.content[:idx-1], l.content[idx:])
		e.moveCursor(ARROW_LEFT)
	}

	e.lines[row].render = e.render(e.lines[row].content)
}

func (e *Editor) insertNewLine() {
	row, idx := e.state.row, e.state.col
	if row < 0 || idx < 0 {
		return
	}
	l := e.lines[row]
	lineBefore := line{l.content[:idx], e.render(l.content[:idx])}
	lineAfter := line{l.content[idx:], e.render(l.content[idx:])}

	e.lines[row] = lineBefore
	e.lines = slices.Insert(e.lines, row+1, lineAfter)
	e.moveCursor(NEW_LINE)
}

// higher level functionality

func (e *Editor) find() {
	indices := FindPositions{}
	for i, l := range e.lines {
		offset := 0
		for {
			idx := strings.Index(l.content[offset:], e.state.findString)
			if idx == -1 {
				break
			} else {
				absoluteIndex := offset + idx
				indices.matches = append(indices.matches, position{i, absoluteIndex})
				offset = absoluteIndex + len(e.state.findString)
			}
		}
	}
	e.state.findMatches = indices
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
		case <-time.After(e.state.inputTimeout):
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
			n := e.state.tabSize - (col % e.state.tabSize)
			c += strings.Repeat(" ", n)
			col += n
		} else {
			c += string(r)
			col++
		}
	}
	return c
}

func (e *Editor) loadFile() {
	file, err := os.OpenFile(e.state.fileName, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		e.shutdown(fmt.Sprintf("error opening/creating file %s: %s", e.state.fileName, err), 1)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		l := scanner.Text()
		e.lines = append(e.lines, line{l, e.render(l)})
	}

	if err := scanner.Err(); err != nil {
		e.shutdown(fmt.Sprintf("error loading file %s: %s", e.state.fileName, err), 1)
	}

	if len(e.lines) == 0 {
		e.lines = append(e.lines, line{"", ""})
	}
}

func (e *Editor) saveFile() (string, int) {
	file, err := os.OpenFile(e.state.fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Sprintf("error opening file %s: %s", e.state.fileName, err), 1
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	s := ""
	for _, l := range e.lines {
		s += l.content + "\n"
	}
	_, err = writer.WriteString(s)
	if err != nil {
		return fmt.Sprintf("error writing file: %s", err), 1
	}
	e.state.writeStatus = fmt.Sprintf("%d bytes written to disc", len(s))
	return e.state.writeStatus, 0
}
