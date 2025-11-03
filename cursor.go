package main

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

// updateRenderedPos updates the rendered position so the cursor is visible
func (c *Cursor) updateRenderedPos(view *View, content string, tabsize int) {
	c.renderedRow = c.row - view.rowOffset + view.topMargin
	c.renderedCol = c.calculateRenderCol(content, tabsize, c.col) + view.leftMargin
}

// calculateRenderCol returns the position of the cursor on the rendered line
func (c *Cursor) calculateRenderCol(content string, tabSize int, col int) int {
	rCol := 0
	for i, r := range content {
		if i >= col {
			break
		}

		if r == TAB {
			rCol += (tabSize - 1) - (rCol % tabSize)
		}
		rCol++
	}

	return rCol
}

func (c *Cursor) setRowTo(newRow int, doc *Document) {
	if newRow == c.row {
		return
	}

	maxValidRow := doc.lineCount() - 1
	if maxValidRow < 0 {
		maxValidRow = 0
	}

	c.row = newRow
	if c.row < 0 {
		c.row = 0
	}
	if c.row > maxValidRow {
		c.row = maxValidRow
	}

	c.col = c.anchor

	targetLength := doc.getLineLength(c.row)
	if c.col > targetLength {
		c.col = targetLength
	}
}
