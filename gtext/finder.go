package gtext

type Finder struct {
	find       bool
	findString string
	matches    FindPositions
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
