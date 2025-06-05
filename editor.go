package main

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
const ESCAPE rune = '\x1b'    // Hex for ASCII Escape
const CSI rune = '\x5b'       // Hex for ASCII '[' (0x5B is 91 decimal)
const CTRL_Q rune = '\x11'    // Hex for ASCII DC1 (Device Control 1)
const CTRL_S rune = '\x13'    // Hex for ASCII DC3 (Device Control 3)
const SPACE rune = '\x20'     // Hex for ASCII Space
const BACKSPACE rune = '\x08' // Hex for ASCII Backspace
const TAB rune = '\x09'       // Hex for ASCII Horizontal Tab
const DELETE rune = '\x7f'    // Hex for ASCII Delete (0x7F is 127 decimal)
const RETURN rune = '\x0d'    // Hex for ASCII Carriage Return (0x0D is 13 decimal)
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

type Editor struct {
	reader    *bufio.Reader
	state     EditorState
	inputChan chan (ReadResult)
	lines     []string
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
	}
}

func NewEditor(r *os.File, fileName string) *Editor {
	initState := NewEditorState(true, fileName)
	return &Editor{
		reader:    bufio.NewReader(r),
		state:     initState,
		inputChan: make(chan ReadResult),
		lines:     []string{},
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
		}
		if r == ESCAPE {
			b, err := e.reader.Peek(1)
			if err != nil {
				e.inputChan <- ReadResult{r: ESCAPE, err: err}
			}
			if len(b) == 1 && b[0] == byte(CSI) {
				_, _, err := e.reader.ReadRune()
				if err != nil {
					e.inputChan <- ReadResult{r: ESCAPE, err: err}
				}
				b1, _, err := e.reader.ReadRune()
				if err != nil {
					e.inputChan <- ReadResult{r: ESCAPE, err: err}
				}
				switch b1 {
				case 'A':
					e.inputChan <- ReadResult{r: ARROW_UP, err: err}
				case 'B':
					e.inputChan <- ReadResult{r: ARROW_DOWN, err: err}
				case 'C':
					e.inputChan <- ReadResult{r: ARROW_RIGHT, err: err}
				case 'D':
					e.inputChan <- ReadResult{r: ARROW_LEFT, err: err}
				case '5':
					b2, _, err := e.reader.ReadRune()
					if err != nil {
						e.inputChan <- ReadResult{r: ESCAPE, err: err}
					}
					if b2 == '~' {
						e.inputChan <- ReadResult{r: PAGE_UP, err: err}
					}
				case '6':
					b2, _, err := e.reader.ReadRune()
					if err != nil {
						e.inputChan <- ReadResult{r: ESCAPE, err: err}
					}
					if b2 == '~' {
						e.inputChan <- ReadResult{r: PAGE_DOWN, err: err}
					}
				case '1', '7':
					b2, _, err := e.reader.ReadRune()
					if err != nil {
						e.inputChan <- ReadResult{r: ESCAPE, err: err}
					}
					if b2 == '~' {
						e.inputChan <- ReadResult{r: HOME, err: err}
					}
				case 'H':
					e.inputChan <- ReadResult{r: HOME, err: err}
				case '4', '8':
					b2, _, err := e.reader.ReadRune()
					if err != nil {
						e.inputChan <- ReadResult{r: ESCAPE, err: err}
					}
					if b2 == '~' {
						e.inputChan <- ReadResult{r: END, err: err}
					}
				case 'F':
					e.inputChan <- ReadResult{r: END, err: err}

				}

			}
		}
		e.inputChan <- ReadResult{r: r, err: err}
	}
}

func (e *Editor) calculateRowOffsetUp() {
	if e.state.row-e.state.maxRowOffset < 0 {
		e.state.maxRowOffset--
	}
}

func (e *Editor) calculateRowOffsetDown() {
	if e.state.row-e.state.maxRowOffset >= e.state.numRow-e.state.botMargin {
		e.state.maxRowOffset++
	}
}

func (e *Editor) moveCursor(r rune) {
	switch r {
	case ARROW_UP:
		if e.state.row > 0 {
			e.state.row--
			e.state.col = e.state.anchor
			if e.state.col > len(e.lines[e.state.row]) {
				e.state.col = len(e.lines[e.state.row])
			}
			e.calculateRowOffsetUp()
		}
	case ARROW_DOWN:
		if e.state.row < len(e.lines)-e.state.botMargin {
			e.state.row++
			e.state.col = e.state.anchor
			if e.state.col > len(e.lines[e.state.row]) {
				e.state.col = len(e.lines[e.state.row])
			}
			e.calculateRowOffsetDown()
		}

	case ARROW_LEFT:
		if e.state.col > 0 {
			e.state.col--
		} else if e.state.row > 0 {
			e.state.row--
			e.state.col = len(e.lines[e.state.row])
			e.calculateRowOffsetUp()
		}
		e.state.anchor = e.state.col
	case ARROW_RIGHT:
		if e.state.col < len(e.lines[e.state.row]) {
			e.state.col++
		} else if e.state.row < len(e.lines)-e.state.botMargin {
			e.state.row++
			e.state.col = 0
			e.calculateRowOffsetDown()
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
		e.state.col = len(e.lines[e.state.row])
	case NEW_LINE:
		e.state.row++
		e.state.col = 0
		e.calculateRowOffsetDown()
	}
}

// rendering

func (e *Editor) makeFooter() string {
	welcomeString := fmt.Sprintf("gtext editor -- version %s", VERSION)
	editorState := fmt.Sprintf(
		"[%d:%d] [lines: %d] [off: %d]",
		e.state.row+1,
		e.state.col+1,
		len(e.lines),
		e.state.maxRowOffset,
	)

	leftPadding := (e.state.numCol-len(welcomeString))/2 - len(editorState)
	rightPadding := (e.state.numCol-len(welcomeString))/2 - len(e.state.fileName)

	s := BLACK_ON_WHITE + "save: Ctrl-S" + strings.Repeat(" ", 5) + "exit: Ctrl-Q" + CLEAR_RIGHT + RESET + "\r\n"
	s += editorState + strings.Repeat(" ", max(leftPadding, 0)) + welcomeString + strings.Repeat(" ", max(0, rightPadding)) + e.state.fileName + CLEAR_RIGHT
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

			s += e.lines[idx] + CLEAR_RIGHT + "\r\n"
		} else {
			s += "~" + CLEAR_RIGHT + "\r\n"
		}
	}
	s += e.makeFooter()
	return s
}

func (e *Editor) refreshScreen() {
	e.state.numCol, e.state.numRow = e.getWindowSize()
	ab := ""
	ab += HIDE_CURSOR
	ab += TOP_LEFT
	ab = e.drawRows(ab)
	ab += fmt.Sprintf(
		"\x1b[%d;%dH",
		e.state.row-e.state.maxRowOffset+e.state.topMargin+1,
		e.state.col+e.state.leftMargin+1)
	ab += SHOW_CURSOR
	fmt.Print(ab)
}

// text editing

func (e *Editor) processKeyPress(r rune) {
	switch r {
	case CTRL_Q:
		e.shutdown("Ctrl+Q", 0)
	case CTRL_S:
		e.saveFile()
	case ARROW_UP, ARROW_DOWN, ARROW_RIGHT, ARROW_LEFT, PAGE_UP, PAGE_DOWN, HOME, END:
		e.moveCursor(r)
	case BACKSPACE, DELETE:
		e.backspace()
	case RETURN:
		e.newLine()
	default:
		if unicode.IsPrint(r) {
			e.write(r)
		}
	}
}

func (e *Editor) write(r rune) {
	row, idx := e.state.row, e.state.col
	if row < 0 || idx < 0 {
		return
	}
	line := e.lines[row]
	if line == "" {
		e.lines[row] = fmt.Sprintf("%c", r)
	} else if idx == 0 {
		e.lines[row] = fmt.Sprintf("%c%s", r, line)
	} else if idx == len(line)+1 {
		e.lines[row] = fmt.Sprintf("%s%c", line, r)
	} else {
		e.lines[row] = fmt.Sprintf("%s%c%s", line[:idx], r, line[idx:])
	}
	e.moveCursor(ARROW_RIGHT)
}

func (e *Editor) backspace() {
	row, idx := e.state.row, e.state.col
	if row < 0 || idx < 0 || len(e.lines) == 0 {
		return
	}
	line := e.lines[row]
	if idx == 0 { //case first position
		if row == 0 { // top row do nothing
			return
		}
		e.moveCursor(ARROW_LEFT)
		if row == len(e.lines)-1 { // last row simple operation
			e.lines[row-1] = fmt.Sprintf("%s%s", e.lines[row-1], line)
			e.lines = e.lines[:row]
		} else { // in the middle need appending
			e.lines[row-1] = fmt.Sprintf("%s%s", e.lines[row-1], line)
			e.lines = append(e.lines[:row], e.lines[row+1:]...)
		}
	} else { // somewhere in the middle
		e.lines[row] = fmt.Sprintf("%s%s", line[:idx-1], line[idx:])
		e.moveCursor(ARROW_LEFT)
	}

}

func (e *Editor) newLine() {
	row, idx := e.state.row, e.state.col
	if row < 0 || idx < 0 {
		return
	}
	line := e.lines[row]
	lineBefore := line[:idx]
	lineAfter := line[idx:]

	newLines := make([]string, 0, len(e.lines)+1)
	newLines = append(newLines, e.lines[:row]...)
	newLines = append(newLines, lineBefore)
	newLines = append(newLines, lineAfter)
	newLines = append(newLines, e.lines[row+1:]...)

	e.lines = newLines
	e.moveCursor(NEW_LINE)
}

// higher level functionality
func (e *Editor) Start() {
	e.loadFile()

	go e.readKeyPresses()

	for {
		select {
		case res := <-e.inputChan:
			if res.err != nil {
				e.shutdown(fmt.Sprintf("%s", res.err), 1)
			}
			e.processKeyPress(res.r)
		case <-time.After(e.state.inputTimeout):
		}
		e.refreshScreen()
	}
}

func (e *Editor) loadFile() {
	file, err := os.OpenFile(e.state.fileName, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		e.shutdown(fmt.Sprintf("error opening/creating file %s: %s", e.state.fileName, err), 1)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		e.lines = append(e.lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		e.shutdown(fmt.Sprintf("error loading file %s: %s", e.state.fileName, err), 1)
	}

	if len(e.lines) == 0 {
		e.lines = append(e.lines, "")
	}
}

func (e *Editor) saveFile() {
	file, err := os.OpenFile(e.state.fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		e.shutdown(fmt.Sprintf("error opening file %s: %s", e.state.fileName, err), 1)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	for _, line := range e.lines {
		_, err := writer.WriteString(fmt.Sprintf("%s\n", line))
		if err != nil {
			e.shutdown(fmt.Sprintf("error writing line %s: %s", line, err), 1)
		}
	}

}
