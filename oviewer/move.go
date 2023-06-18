package oviewer

import (
	"context"
	"log"
	"regexp"
)

// moveLine moves to the specified line.
func (m *Document) moveLine(lN int) int {
	lN = min(lN, m.BufEndNum())
	m.topLN = lN
	m.topLX = 0
	return lN
}

// moveTop moves to the top.
func (m *Document) moveTop() {
	m.moveLine(m.BufStartNum())
}

// Go to the top line.
// Called from a EventKey.
func (root *Root) moveTop() {
	root.Doc.moveTop()
}

// Go to the bottom line.
// Called from a EventKey.
func (root *Root) moveBottom() {
	root.Doc.topLX, root.Doc.topLN = root.bottomLineNum(root.Doc.BufEndNum())
}

// Move to the nth wrapping line of the specified line.
func (root *Root) moveLineNth(lN int, nTh int) (int, int) {
	lN = root.Doc.moveLine(lN)
	if !root.Doc.WrapMode {
		return lN, 0
	}
	listX := root.leftMostX(lN + root.Doc.firstLine())
	if len(listX) == 0 {
		return lN, 0
	}

	if nTh >= len(listX) {
		nTh = len(listX) - 1
	}

	root.Doc.topLN = lN
	root.Doc.topLX = listX[nTh]

	return lN, nTh
}

// Move up one screen.
// Called from a EventKey.
func (root *Root) movePgUp() {
	root.resetSelect()
	defer root.releaseEventBuffer()

	root.moveNumUp(root.statusPos - root.headerLen)
	if root.Doc.topLN < root.Doc.BufStartNum() {
		root.Doc.moveTop()
	}
}

// Moves down one screen.
// Called from a EventKey.
func (root *Root) movePgDn() {
	root.resetSelect()
	defer root.releaseEventBuffer()

	m := root.Doc
	y := m.bottomLN - m.firstLine()
	x := m.bottomLX
	root.limitMoveDown(x, y)
}

func (root *Root) limitMoveDown(x int, y int) {
	m := root.Doc
	if y+root.scr.vHeight >= m.BufEndNum()-m.SkipLines {
		tx, tn := root.bottomLineNum(m.BufEndNum())
		if y > tn || (y == tn && x > tx) {
			if m.topLN < tn || (m.topLN == tn && m.topLX < tx) {
				m.topLN = tn
				m.topLX = tx
			}
			return
		}
	}
	m.topLN = y
	m.topLX = x
}

// Moves up half a screen.
// Called from a EventKey.
func (root *Root) moveHfUp() {
	root.resetSelect()
	defer root.releaseEventBuffer()

	root.moveNumUp((root.statusPos - root.headerLen) / 2)
	if root.Doc.topLN < root.Doc.BufStartNum() {
		root.Doc.moveTop()
	}
}

// Moves down half a screen.
// Called from a EventKey.
func (root *Root) moveHfDn() {
	root.resetSelect()
	defer root.releaseEventBuffer()

	root.moveNumDown((root.statusPos - root.headerLen) / 2)
}

// numOfWrap returns the number of wrap from lX and lY.
func (root *Root) numOfWrap(lX int, lY int) int {
	listX := root.leftMostX(lY)
	if len(listX) == 0 {
		return 0
	}
	return numOfSlice(listX, lX)
}

// numOfSlice returns what number x is in slice.
func numOfSlice(listX []int, x int) int {
	for n, v := range listX {
		if v >= x {
			return n
		}
	}
	return len(listX) - 1
}

// numOfReverseSlice returns what number x is from the back of slice.
func numOfReverseSlice(listX []int, x int) int {
	for n := len(listX) - 1; n >= 0; n-- {
		if listX[n] <= x {
			return n
		}
	}
	return 0
}

// Moves up by the specified number of y.
func (root *Root) moveNumUp(moveY int) {
	m := root.Doc
	if !m.WrapMode {
		m.topLN -= moveY
		return
	}

	// WrapMode
	num := m.topLN + m.firstLine()
	m.topLX, num = root.findNumUp(m.topLX, num, moveY)
	m.topLN = num - m.firstLine()
}

// Moves down by the specified number of y.
func (root *Root) moveNumDown(moveY int) {
	m := root.Doc
	num := m.topLN + m.firstLine()
	if !m.WrapMode {
		root.limitMoveDown(0, num+moveY)
		return
	}

	// WrapMode
	x := m.topLX
	listX := root.leftMostX(num)
	n := numOfReverseSlice(listX, x)

	for y := 0; y < moveY; y++ {
		if n >= len(listX) {
			num++
			if num > m.BufEndNum() {
				break
			}
			listX = root.leftMostX(num)
			n = 0
		}
		x = 0
		if len(listX) > 0 && n < len(listX) {
			x = listX[n]
		}
		n++
	}

	root.limitMoveDown(x, num-m.Header)
}

// Move up one line.
// Called from a EventKey.
func (root *Root) moveUp() {
	root.moveUpN(1)
}

// Move up by n amount.
func (root *Root) moveUpN(n int) {
	root.resetSelect()
	defer root.releaseEventBuffer()

	m := root.Doc
	if m.topLN <= m.BufStartNum() && m.topLX == 0 {
		return
	}

	if !m.WrapMode {
		m.topLN -= n
		m.topLX = 0
		return
	}

	// WrapMode.
	// Same line.
	if m.topLX > 0 {
		listX := root.leftMostX(m.topLN + m.firstLine())
		for n, x := range listX {
			if x >= m.topLX {
				m.topLX = listX[n-1]
				return
			}
		}
	}

	// Previous line.
	m.topLN -= n
	start := m.BufStartNum()
	if m.topLN < start {
		m.topLN = start
		m.topLX = 0
		return
	}
	listX := root.leftMostX(m.topLN + m.firstLine())
	if len(listX) > 0 {
		m.topLX = listX[len(listX)-1]
		return
	}
	m.topLX = 0
}

// Move down one line.
// Called from a EventKey.
func (root *Root) moveDown() {
	root.moveDownN(1)
}

// Move down by n amount.
func (root *Root) moveDownN(n int) {
	root.resetSelect()
	defer root.releaseEventBuffer()

	m := root.Doc
	num := m.topLN

	if !m.WrapMode {
		num += n
		root.limitMoveDown(0, num)
		return
	}

	// WrapMode
	listX := root.leftMostX(m.topLN + m.firstLine())

	for _, x := range listX {
		if x > m.topLX {
			root.limitMoveDown(x, num)
			return
		}
	}

	// Next line.
	num += n
	m.topLX = 0
	root.limitMoveDown(m.topLX, num)
}

// nextSection moves down to the next section's delimiter.
func (root *Root) nextSection() {
	// Move by page, if there is no section delimiter.
	if root.Doc.SectionDelimiter == "" {
		root.movePgDn()
		return
	}

	root.resetSelect()
	defer root.releaseEventBuffer()

	m := root.Doc
	num := m.topLN + m.firstLine() + (1 - m.SectionStartPosition)
	searcher := NewSearcher(m.SectionDelimiter, m.SectionDelimiterReg, true, true)
	ctx := context.Background()
	defer ctx.Done()
	n, err := m.SearchLine(ctx, searcher, num)
	if err != nil {
		// Last section or no section.
		root.setMessage("no next section")
		root.movePgDn()
		return
	}
	root.Doc.moveLine((n - m.firstLine()) + m.SectionStartPosition)
}

// prevSection moves up to the delimiter of the previous section.
func (root *Root) prevSection() {
	// Move by page, if there is no section delimiter.
	if root.Doc.SectionDelimiter == "" {
		root.movePgUp()
		return
	}

	root.resetSelect()
	defer root.releaseEventBuffer()

	m := root.Doc
	num := m.topLN + m.firstLine() - (1 + m.SectionStartPosition)
	searcher := NewSearcher(m.SectionDelimiter, m.SectionDelimiterReg, true, true)
	ctx := context.Background()
	defer ctx.Done()
	n, err := m.BackSearchLine(ctx, searcher, num)
	if err != nil {
		m.moveTop()
		return
	}
	n = (n - m.firstLine()) + m.SectionStartPosition
	n = max(n, m.BufStartNum())
	m.moveLine(n)
}

// lastSection moves to the last section.
func (root *Root) lastSection() {
	root.resetSelect()

	m := root.Doc
	// +1 to avoid if the bottom line is a session delimiter.
	num := m.BufEndNum() - 2
	searcher := NewSearcher(m.SectionDelimiter, m.SectionDelimiterReg, true, true)
	ctx := context.Background()
	defer ctx.Done()
	n, err := m.BackSearchLine(ctx, searcher, num)
	if err != nil {
		log.Printf("last section:%v", err)
		return
	}
	n = (n - m.firstLine()) + m.SectionStartPosition
	m.moveLine(n)
}

// Move to the left.
// Called from a EventKey.
func (root *Root) moveLeft() {
	root.moveLeftN(1)
}

// Move to the right.
// Called from a EventKey.
func (root *Root) moveRight() {
	root.moveRightN(1)
}

// Move left by n amount.
func (root *Root) moveLeftN(n int) {
	root.resetSelect()
	defer root.releaseEventBuffer()

	m := root.Doc
	if !m.ColumnMode {
		if m.WrapMode {
			return
		}
		m.x -= n
		if m.x < root.minStartX {
			m.x = root.minStartX
		}
		return
	}

	if m.columnCursor <= 0 {
		return
	}
	cursor := m.columnCursor - n
	x, err := root.columnX(cursor)
	if err != nil {
		root.debugMessage(err.Error())
		return
	}
	m.x = x
	m.columnCursor = cursor
}

// Move right by n amount.
func (root *Root) moveRightN(n int) {
	root.resetSelect()
	defer root.releaseEventBuffer()

	m := root.Doc
	if !m.ColumnMode {
		if m.WrapMode {
			return
		}
		end := root.endRight()
		if end < m.x+n {
			m.x = end
		} else {
			m.x += n
		}
		return
	}

	cursor := m.columnCursor + n
	x, err := root.columnX(cursor)
	if err != nil {
		root.debugMessage(err.Error())
		m.x = x
		return
	}
	m.x = x
	m.columnCursor = cursor
}

func (root *Root) cursorCorrection(cursor int) int {
	m := root.Doc
	if m.WrapMode {
		return cursor
	}

	if m.ColumnWidth {
		return cursorFromPosition(m.columnWidths, cursor, m.x, m.x+root.scr.vWidth)
	}

	for i := 0; i < m.firstLine()+10; i++ {
		line, valid := m.getLineC(m.topLN+m.firstLine()+i, m.TabWidth)
		if !valid {
			continue
		}
		widths := widthsDelimiter(line.str, m.ColumnDelimiter, m.ColumnDelimiterReg)

		if len(widths) <= cursor {
			continue
		}
		return cursorFromPosition(widths, cursor, m.x, m.x+root.scr.vWidth)
	}
	return cursor
}

func cursorFromPosition(widths []int, cursor int, start int, end int) int {
	if len(widths) > cursor+1 && widths[cursor] < start && widths[cursor+1] < end {
		for n, wx := range widths {
			if wx >= start {
				return n
			}
		}
		return len(widths) - 1
	}
	if cursor > 1 && widths[cursor] > end && widths[cursor-1] > start {
		for n := len(widths) - 1; n >= 0; n-- {
			if widths[n] < end {
				return n
			}
		}
	}
	return cursor
}

func widthsDelimiter(str string, delimiter string, delimiterReg *regexp.Regexp) []int {
	indexes := allIndex(str, delimiter, delimiterReg)
	if len(indexes) == 0 {
		return nil
	}

	widths := make([]int, len(indexes)+1)
	widths[0] = 0
	for i := 0; i < len(indexes); i++ {
		widths[i+1] = indexes[i][1] + 1
	}
	return widths
}

// columnDelimiterX returns the columnCursor and actual x from columnCursor.
func (root *Root) columnX(cursor int) (int, error) {
	if root.Doc.ColumnWidth {
		return root.columnWidthX(cursor)
	}
	return root.columnDelimiterX(cursor)
}

// columnWidthX returns the columnCursor and actual x from columnCursor.
func (root *Root) columnWidthX(cursor int) (int, error) {
	m := root.Doc
	if cursor <= 0 {
		return 0, nil
	}
	if len(m.columnWidths) >= cursor {
		x := m.columnWidths[cursor-1]
		return x, nil
	}
	return 0, ErrNoDelimiter
}

// columnDelimiterX returns the columnCursor and actual x from columnCursor.
func (root *Root) columnDelimiterX(cursor int) (int, error) {
	const columnEdge = 2
	m := root.Doc
	if cursor <= 0 {
		return 0, nil
	}

	maxCursor := 0
	// m.firstLine()+10 = Maximum columnMode target.
	for i := 0; i < m.firstLine()+10; i++ {
		line, valid := m.getLineC(m.topLN+m.firstLine()+i, m.TabWidth)
		if !valid {
			continue
		}
		indexes := widthsDelimiter(line.str, m.ColumnDelimiter, m.ColumnDelimiterReg)
		maxCursor = max(maxCursor, len(indexes))
		if len(indexes) < cursor {
			continue
		}

		if len(indexes) == cursor {
			// right over column.
			end := line.pos.x(len(line.str))
			if end-m.x < root.scr.vWidth { // don't scroll.
				return m.x, nil
			}
			if m.x+root.scr.vWidth < end-root.scr.vWidth {
				return m.x + root.scr.vWidth, ErrNoColumn
			}
			return end - root.scr.vWidth + columnEdge, ErrNoColumn
		}

		start, end := 0, 0
		if cursor+1 > len(indexes)-1 {
			// right edge.
			start = line.pos.x(indexes[len(indexes)-1])
			end = line.pos.x(len(line.str))
		} else {
			start = line.pos.x(indexes[cursor])
			end = line.pos.x(indexes[cursor+1])
		}

		if start < m.x {
			return start - columnEdge, nil
		}
		if end > m.x {
			if end-m.x < root.scr.vWidth { // don't scroll.
				return m.x, nil
			}
			if end-start < root.scr.vWidth {
				if (end-root.scr.vWidth)-m.x > root.scr.vWidth {
					return m.x + root.scr.vWidth, ErrOverScreen
				}

				if (cursor == len(indexes)-1) && (end == start) {
					return end - root.scr.vWidth, nil
				}
				return end - root.scr.vWidth, nil
			}
			if end == start {
				return m.x, ErrNoColumn
			}
		}
		return start - columnEdge, nil
	}

	if maxCursor > 0 {
		return m.x, ErrNoColumn
	}
	return 0, ErrNoDelimiter
}

// Move to the left by half a screen.
// Called from a EventKey.
func (root *Root) moveHfLeft() {
	root.resetSelect()
	defer root.releaseEventBuffer()

	m := root.Doc

	if m.WrapMode {
		return
	}
	moveSize := (root.scr.vWidth / 2)
	if m.x > 0 && (m.x-moveSize) < 0 {
		m.x = 0
		return
	}

	m.x -= moveSize
	if m.x < root.minStartX {
		m.x = root.minStartX
	}
}

// Move to the right by half a screen.
// Called from a EventKey.
func (root *Root) moveHfRight() {
	root.resetSelect()
	defer root.releaseEventBuffer()

	m := root.Doc

	if m.WrapMode {
		return
	}
	if m.x < 0 {
		m.x = 0
		return
	}

	m.x += (root.scr.vWidth / 2)
}

// moveBeginLeft moves to the beginning of the line.
func (root *Root) moveBeginLeft() {
	m := root.Doc

	if m.WrapMode {
		return
	}
	m.x = 0
}

// moveEndRight moves to the end of the line.
// Move so that the end of the currently displayed line is visible.
func (root *Root) moveEndRight() {
	m := root.Doc
	if m.WrapMode {
		return
	}
	m.x = root.endRight()
}

func (root *Root) endRight() int {
	m := root.Doc
	x := 0
	for _, line := range root.scr.numbers {
		lY := line.number
		lc, err := m.contents(lY, m.TabWidth)
		if err != nil {
			continue
		}
		x = max(x, len(lc)-1)
	}
	return x - ((root.scr.vWidth - root.scr.startX) - 1)
}

// bottomLineNum returns the display start line
// when the last line number as an argument.
func (root *Root) bottomLineNum(lN int) (int, int) {
	if lN < root.headerLen {
		return 0, 0
	}

	height := (root.scr.vHeight - root.headerLen) - 2
	if !root.Doc.WrapMode {
		return 0, lN - (height + root.Doc.firstLine())
	}
	// WrapMode
	lX, lN := root.findNumUp(0, lN, height)
	return lX, lN - root.Doc.firstLine()
}

// findNumUp finds lX, lN when the number of lines is moved up from lX, lN.
func (root *Root) findNumUp(lX int, lN int, upY int) (int, int) {
	listX := root.leftMostX(lN)
	n := numOfSlice(listX, lX)

	for y := upY; y > 0; y-- {
		if n <= 0 {
			lN--
			listX = root.leftMostX(lN)
			n = len(listX)
		}
		if n > 0 {
			lX = listX[n-1]
		} else {
			lX = 0
		}
		n--
	}
	return lX, lN
}

// leftMostX returns a list of left - most x positions when wrapping.
// Returns nil if there is no line number.
func (root *Root) leftMostX(lN int) []int {
	lc, err := root.Doc.contents(lN, root.Doc.TabWidth)
	if err != nil {
		return nil
	}

	listX := make([]int, 0, (len(lc)/root.scr.vWidth)+1)
	width := (root.scr.vWidth - root.scr.startX)

	listX = append(listX, 0)
	for n := width; n < len(lc); n += width {
		if lc[n-1].width == 2 {
			n--
		}
		listX = append(listX, n)
	}
	return listX
}
