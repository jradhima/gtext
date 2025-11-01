package main

import (
	"bufio"
	"fmt"
	"os"
)

type Cursor struct {
	row, col                 int
	renderedRow, renderedCol int
	anchor                   int
}

func NewCursor(row, col int) *Cursor {
	cursor := Cursor{
		row: row,
		col: col,
	}
	return &cursor
}

// getPosition returns the position of the cursor on the screen
func (c *Cursor) getPosition() (int, int, error) {
	_, err := fmt.Print(CURSOR_POSITION)
	if err != nil {
		return 0, 0, fmt.Errorf("error requesting cursor position: %w", err)
	}

	reader := bufio.NewReader(os.Stdin)
	b, err := reader.ReadBytes('R')
	if err != nil {
		return 0, 0, fmt.Errorf("error reading stdin for cursor: %w", err)
	} else if b[0] != '\x1b' || b[1] != '[' {
		return 0, 0, fmt.Errorf("cursor position return not valid: %v", b)
	}

	var nrow, ncol int
	_, err = fmt.Sscanf(string(b[1:]), "[%d;%dR", &nrow, &ncol)
	if err != nil {
		return 0, 0, fmt.Errorf("error parsing cursor position return: %w", err)
	}
	return nrow, ncol, nil
}

func (c *Cursor) coords() (int, int) {
	return c.row, c.col
}

// ScreenCoords return individual 1-indexed row and col values
func (c *Cursor) screenCoords() (int, int) {
	return c.renderedRow + 1, c.renderedCol + 1
}

func (c *Cursor) moveTo(row, col int) {
	c.row = row
	c.col = col
}
