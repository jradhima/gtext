package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

const (
	ErrRowOutOfBounds = gtextError("requested line number not in document")
	ErrColOutOfBounds = gtextError("requested column position not in line")
)

type Document struct {
	fileName string
	lines    []line
	dirty    bool
	config   *Config
}

type line struct {
	content string
	render  string
}

func NewDocument(fileName string, config *Config) *Document {
	doc := Document{
		fileName: fileName,
		lines:    []line{{"", ""}},
		dirty:    false,
		config:   config,
	}
	return &doc
}

func (d *Document) lineCount() int {
	return len(d.lines)
}

func (d *Document) getLineLength(row int) int {
	if row < 0 || row >= len(d.lines) {
		return 0
	}
	return len([]rune(d.lines[row].content))
}

// renderLine renders the view of the raw content of a line
// including tabs, etc
func (d *Document) renderLine(s string) string {
	var builder strings.Builder
	col := 0
	for _, r := range s {
		if r == TAB {
			n := d.config.TabSize - (col % d.config.TabSize)
			builder.WriteString(strings.Repeat(" ", n))
			col += n
		} else {
			builder.WriteString(string(r))
			col++
		}
	}
	return builder.String()
}

// checkPosition checks that the provided position is a valid one
func (d *Document) checkPosition(row, col int) error {
	lineCount := d.lineCount()
	if row < 0 || row > lineCount {
		return ErrRowOutOfBounds
	}

	if row == lineCount {
		if col != 0 {
			return ErrColOutOfBounds
		}
		return nil
	}

	lineLength := d.getLineLength(row)
	if col < 0 || col > lineLength {
		return ErrColOutOfBounds
	}

	return nil
}

// getLine fetches the raw content of a line
func (d *Document) getLine(row int) (string, error) {
	lineCount := d.lineCount()

	if row < 0 || row >= lineCount {
		return "", ErrRowOutOfBounds
	}
	return d.lines[row].content, nil
}

// addLine adds a line at position row
func (d *Document) addLine(row int, content string) error {
	err := d.checkPosition(row, 0)
	if err != nil {
		return fmt.Errorf("could not add line at row %d: %w", row, err)
	}
	l := line{content: content, render: d.renderLine(content)}
	d.lines = slices.Insert(d.lines, row, l)
	return nil
}

// removeLine removes line at position row
func (d *Document) removeLine(row int) error {
	err := d.checkPosition(row, 0)
	if err != nil {
		return fmt.Errorf("could not add line at row %d: %w", row, err)
	}
	d.lines = slices.Delete(d.lines, row, row+1)
	return nil
}

// replaceLine replaces the content and render of a line
func (d *Document) replaceLine(row int, content string) error {
	err := d.checkPosition(row, 0)
	if err != nil {
		return fmt.Errorf("could not delete line at row %d: %w", row, err)
	}
	l := line{content: content, render: d.renderLine(content)}
	d.lines[row] = l
	return nil
}

// insertRune inserts a rune to the content of the line at the specified location
// does not handle newline characters
func (d *Document) insertRune(row, col int, r rune) error {
	err := d.checkPosition(row, col)
	if err != nil {
		return fmt.Errorf("could not insert rune %c at row %d, col %d: %w", r, row, col, err)
	}

	content, err := d.getLine(row)
	if err != nil {
		return err
	}

	contentRunes := []rune(content)
	newContentRunes := slices.Insert(contentRunes, col, r)

	err = d.replaceLine(row, string(newContentRunes))
	if err != nil {
		return err
	}
	return nil
}

// deleteRune deletes the rune at the specified location
// does not handle merging of rows
func (d *Document) deleteRune(row, col int) error {
	err := d.checkPosition(row, col)
	if err != nil {
		return fmt.Errorf("could not delete rune at row %d, col %d: %w", row, col, err)
	}

	lineContent, err := d.getLine(row)
	if err != nil {
		return fmt.Errorf("could not get line at row %d: %w", row, err)
	}
	lineRunes := []rune(lineContent)
	modifiedLineRunes := slices.Delete(lineRunes, col-1, col)
	err = d.replaceLine(row, string(modifiedLineRunes))
	if err != nil {
		return fmt.Errorf("could not replace line at row %d: %w", row, err)
	}

	return nil
}

// inserts a new empty line at the specified position
func (d *Document) insertNewLine(row, col int) (newRow, newCol int, err error) {
	err = d.checkPosition(row, col)
	if err != nil {
		return 0, 0, fmt.Errorf("could not insert new line at row %d, col %d: %w", row, col, err)
	}

	if col == 0 {
		err = d.addLine(row, "")
		if err != nil {
			return 0, 0, fmt.Errorf("could not add empty line at current row %d: %w", row, err)
		}
		return row + 1, 0, nil
	}

	l := d.getLineLength(row)
	if col == l {
		err = d.addLine(row+1, "")
		if err != nil {
			return 0, 0, fmt.Errorf("could not add empty line at next row %d: %w", row+1, err)
		}
		return row + 1, 0, nil
	}

	lineContent, err := d.getLine(row)
	if err != nil {
		return 0, 0, fmt.Errorf("could not get line at row %d: %w", row, err)
	}

	lineRunes := []rune(lineContent)
	currentLineRunes := lineRunes[:col]
	nextLineRunes := lineRunes[col:]

	err = d.replaceLine(row, string(currentLineRunes))
	if err != nil {
		return 0, 0, fmt.Errorf("could not replace line at current row %d: %w", row, err)
	}
	err = d.addLine(row+1, string(nextLineRunes))
	if err != nil {
		return 0, 0, fmt.Errorf("could not add remainder line at next row %d: %w", row, err)
	}
	return row + 1, 0, nil
}

// mergeLines handles deleting the newline character at the beginning of a line
func (d *Document) mergeLines(row int) (newRow, newCol int, err error) {
	err = d.checkPosition(row, 0)
	if err != nil {
		return 0, 0, fmt.Errorf("could not merge lines at row %d: %w", row, err)
	}
	if row == 0 {
		return 0, 0, nil
	}
	lineContent, err := d.getLine(row)
	if err != nil {
		return 0, 0, fmt.Errorf("could not get line at row %d: %w", row, err)
	}
	prevLineContent, err := d.getLine(row - 1)
	if err != nil {
		return 0, 0, fmt.Errorf("could not get previous line at row %d: %w", row-1, err)
	}
	err = d.replaceLine(row-1, prevLineContent+lineContent)
	if err != nil {
		return 0, 0, fmt.Errorf("could not merge lines at rows %d, %d: %w", row-1, row, err)
	}
	err = d.removeLine(row)
	if err != nil {
		return 0, 0, fmt.Errorf("could not remove line at row %d: %w", row, err)
	}
	prevLineLength := len([]rune(prevLineContent))
	return row - 1, prevLineLength, nil
}

// Load reads the file content into the document, replacing the contents
func (d *Document) Load(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	var lines []line
	for scanner.Scan() {
		content := scanner.Text()
		lines = append(lines, line{content: content, render: d.renderLine(content)})
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error while loading file: %w", err)
	}
	if len(lines) == 0 {
		d.lines = []line{{"", ""}}
	} else {
		d.lines = lines
	}
	return nil
}

// LoadFromDisk reads the file content from the filename
func (d *Document) LoadFromDisk() error {
	file, err := os.OpenFile(d.fileName, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("error opening file %s from disk: %w", d.fileName, err)
	}
	defer file.Close()

	err = d.Load(file)
	if err != nil {
		return fmt.Errorf("failed to process content of file %s: %w", d.fileName, err)
	}
	return nil
}

// Save writes the contents of the document into the writer
func (d *Document) Save(w io.Writer) (int, error) {
	var builder strings.Builder
	for _, line := range d.lines {
		builder.WriteString(line.content)
		builder.WriteRune('\n')
	}
	n, err := w.Write([]byte(builder.String()))
	if err != nil {
		return 0, fmt.Errorf("error writing file: %w", err)
	}
	return n, nil
}

// SaveToDisk writes the contents to the document's filename
func (d *Document) SaveToDisk() (int, error) {
	file, err := os.OpenFile(d.fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return 0, fmt.Errorf("error opening file %s from disk: %w", d.fileName, err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	n, err := d.Save(writer)
	if err != nil {
		return 0, fmt.Errorf("failed to write content of file %s: %w", d.fileName, err)
	}
	return n, nil
}
