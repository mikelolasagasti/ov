package oviewer

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/gdamore/tcell/v2"
)

var defaultStyle = tcell.StyleDefault

// draw is the main routine that draws the screen.
func (root *Root) draw(ctx context.Context) {
	m := root.Doc

	if root.scr.vHeight == 0 {
		m.topLN = 0
		root.drawStatus()
		root.Show()
		return
	}

	// Prepare the contents.
	root.prepareContents(ctx)

	// Header.
	lN := root.drawHeader()

	lX := 0
	if m.WrapMode {
		lX = m.topLX
	}
	lN = m.topLN + lN

	// Body.
	lX, lN = root.drawBody(lX, lN)

	// Section header.
	root.drawSectionHeader()

	m.bottomLN = max(lN, 0)
	m.bottomLX = lX

	if root.mouseSelect {
		root.drawSelect(root.x1, root.y1, root.x2, root.y2, true)
	}

	root.drawStatus()
	root.Show()
}

// drawHeader draws header.
func (root *Root) drawHeader() int {
	m := root.Doc

	if m.headerHeight <= 0 {
		return m.SkipLines
	}

	// lX is the logical x position of Contents.
	lX := 0
	// lN is a logical line number.
	lN := m.SkipLines
	// wrapNum is the number of wrapped lines.
	wrapNum := 0
	for y := 0; y < m.headerHeight && lN < m.firstLine(); y++ {
		line, ok := root.scr.contents[lN]
		if !ok {
			panic(fmt.Sprintf("line is not found %d", lN))
		}
		root.scr.numbers[y] = newLineNumber(lN, wrapNum)
		root.blankLineNumber(y)

		lX, lN = root.drawLine(y, lX, lN, line.lc)
		// header style.
		root.yStyle(y, root.StyleHeader)

		wrapNum++
		if lX == 0 {
			wrapNum = 0
		}
	}

	return lN
}

// drawBody draws body.
func (root *Root) drawBody(lX int, lN int) (int, int) {
	m := root.Doc

	wrapNum := m.numOfWrap(lX, lN)
	for y := m.headerHeight; y < root.scr.vHeight-statusLine; y++ {
		line, ok := root.scr.contents[lN]
		if !ok {
			panic(fmt.Sprintf("line is not found %d", lN))
		}
		root.scr.numbers[y] = newLineNumber(lN, wrapNum)
		root.drawLineNumber(lN, y, line.valid)

		nextLX, nextLN := root.drawLine(y, lX, lN, line.lc)
		if line.valid {
			root.coordinatesStyle(lN, y, line.str)
		}

		wrapNum++
		if nextLX == 0 {
			wrapNum = 0
		}

		lX = nextLX
		lN = nextLN
	}
	return lX, lN
}

// drawSectionHeader draws section header.
// The section header overrides the body.
func (root *Root) drawSectionHeader() {
	m := root.Doc

	if m.sectionHeaderHeight <= 0 {
		return
	}

	lX := 0
	lN := root.scr.sectionHeaderLN
	wrapNum := 0
	for y := m.headerHeight; y < m.headerHeight+m.sectionHeaderHeight; y++ {
		line, ok := root.scr.contents[lN]
		if !ok {
			panic(fmt.Sprintf("line is not found %d", lN))
		}
		root.scr.numbers[y] = newLineNumber(lN, wrapNum)
		root.drawLineNumber(lN, y, line.valid)

		nextLX, nextLN := root.drawLine(y, lX, lN, line.lc)
		// section header style.
		root.yStyle(y, root.StyleSectionLine)
		// markstyle is displayed above the section header.
		markStyleWidth := min(root.scr.vWidth, m.general.MarkStyleWidth)
		root.markStyle(lN, y, markStyleWidth)
		// Underline search lines when they overlap in section headers.
		if lN == m.lastSearchNum {
			root.yStyle(y, OVStyle{Underline: true})
		}

		wrapNum++
		if nextLX == 0 {
			wrapNum = 0
		}

		lX = nextLX
		lN = nextLN
	}
}

// drawWrapLine wraps and draws the contents and returns the next drawing position.
func (root *Root) drawLine(y int, lX int, lN int, lc contents) (int, int) {
	if root.Doc.WrapMode {
		return root.drawWrapLine(y, lX, lN, lc)
	}

	return root.drawNoWrapLine(y, root.Doc.x, lN, lc)
}

// drawWrapLine wraps and draws the contents and returns the next drawing position.
func (root *Root) drawWrapLine(y int, lX int, lN int, lc contents) (int, int) {
	if lX < 0 {
		log.Printf("Illegal lX:%d", lX)
		return 0, 0
	}

	for x := 0; ; x++ {
		if lX+x >= len(lc) {
			// EOL
			root.clearEOL(root.scr.startX+x, y)
			lX = 0
			lN++
			break
		}
		content := lc[lX+x]
		if x+root.scr.startX+content.width > root.scr.vWidth {
			// Right edge.
			root.clearEOL(root.scr.startX+x, y)
			lX += x
			break
		}
		root.Screen.SetContent(root.scr.startX+x, y, content.mainc, content.combc, content.style)
	}

	return lX, lN
}

// drawNoWrapLine draws contents without wrapping and returns the next drawing position.
func (root *Root) drawNoWrapLine(y int, startX int, lN int, lc contents) (int, int) {
	startX = max(startX, root.minStartX)
	for x := 0; root.scr.startX+x < root.scr.vWidth; x++ {
		if startX+x >= len(lc) {
			// EOL
			root.clearEOL(root.scr.startX+x, y)
			break
		}
		content := DefaultContent
		if startX+x >= 0 {
			content = lc[startX+x]
		}
		root.Screen.SetContent(root.scr.startX+x, y, content.mainc, content.combc, content.style)
	}
	lN++

	return startX, lN
}

// blankLineNumber should be blank for the line number.
func (root *Root) blankLineNumber(y int) {
	if !root.Doc.LineNumMode {
		return
	}
	if root.scr.startX <= 0 {
		return
	}
	numC := StrToContents(strings.Repeat(" ", root.scr.startX-1), root.Doc.TabWidth)
	root.setContentString(0, y, numC)
}

// drawLineNumber draws the line number.
func (root *Root) drawLineNumber(lN int, y int, valid bool) {
	m := root.Doc
	if !m.LineNumMode {
		return
	}
	if !valid {
		root.blankLineNumber(y)
		return
	}
	if root.scr.startX <= 0 {
		return
	}

	number := lN
	if m.lineNumMap != nil {
		n, ok := m.lineNumMap.LoadForward(number)
		if ok {
			number = n
		}
	}
	number = number - m.firstLine() + 1

	// Line numbers start at 1 except for skip and header lines.
	numC := StrToContents(fmt.Sprintf("%*d", root.scr.startX-1, number), m.TabWidth)
	for i := 0; i < len(numC); i++ {
		numC[i].style = applyStyle(defaultStyle, root.StyleLineNumber)
	}
	root.setContentString(0, y, numC)
}

// setContentString is a helper function that draws a string with setContent.
func (root *Root) setContentString(vx int, vy int, lc contents) {
	screen := root.Screen
	for x, content := range lc {
		screen.SetContent(vx+x, vy, content.mainc, content.combc, content.style)
	}
	screen.SetContent(vx+len(lc), vy, 0, nil, defaultStyle.Normal())
}

// clearEOL clears from the specified position to the right end.
func (root *Root) clearEOL(x int, y int) {
	for ; x < root.scr.vWidth; x++ {
		root.Screen.SetContent(x, y, ' ', nil, defaultStyle)
	}
}

// clearY clear the specified line.
func (root *Root) clearY(y int) {
	root.clearEOL(0, y)
}

// coordinatesStyle applies the style of the coordinates.
func (root *Root) coordinatesStyle(lN int, y int, str string) {
	root.alternateRowsStyle(lN, y)
	root.sectionLineHighlight(y, str)
	markStyleWidth := min(root.scr.vWidth, root.Doc.general.MarkStyleWidth)
	root.markStyle(lN, y, markStyleWidth)
	if root.Doc.jumpTargetHeight != 0 && root.Doc.headerHeight+root.Doc.jumpTargetHeight == y {
		root.yStyle(y, root.StyleJumpTargetLine)
	}
}

// alternateRowsStyle applies from beginning to end of line.
func (root *Root) alternateRowsStyle(lN int, y int) {
	if root.Doc.AlternateRows {
		if (lN)%2 == 1 {
			root.yStyle(y, root.StyleAlternate)
		}
	}
}

// yStyle applies the style from the left edge to the right edge of the physical line.
// Apply styles to the screen.
func (root *Root) yStyle(y int, s OVStyle) {
	for x := 0; x < root.scr.vWidth; x++ {
		r, c, ts, _ := root.GetContent(x, y)
		root.Screen.SetContent(x, y, r, c, applyStyle(ts, s))
	}
}

// markStyle applies the style from the left edge to the specified width.
func (root *Root) markStyle(lN int, y int, width int) {
	m := root.Doc
	if contains(m.marked, lN) {
		for x := 0; x < width; x++ {
			r, c, style, _ := root.GetContent(x, y)
			root.Screen.SetContent(x, y, r, c, applyStyle(style, root.StyleMarkLine))
		}
	}
}

// sectionLineHighlight applies the style of the section line highlight.
func (root *Root) sectionLineHighlight(y int, str string) {
	m := root.Doc
	if m.SectionDelimiter == "" {
		return
	}
	if m.SectionDelimiterReg == nil {
		log.Printf("Regular expression is not set: %s", m.SectionDelimiter)
		return
	}

	root.scr.sectionHeaderLeft--
	if root.scr.sectionHeaderLeft > 0 {
		root.yStyle(y, root.StyleSectionLine)
	}
	if m.SectionDelimiterReg.MatchString(str) {
		root.yStyle(y, root.StyleSectionLine)
		root.scr.sectionHeaderLeft = m.SectionHeaderNum
	}
}
