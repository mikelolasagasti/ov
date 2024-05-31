package oviewer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
)

// statusLine is the number of lines in the status bar.
const statusLine = 1

// sectionTimeOut is the section header search timeout(in milliseconds) period.
const sectionTimeOut = 1000

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

	// Header
	lN := root.drawHeader()

	lX := 0
	if m.WrapMode {
		lX = m.topLX
	}
	lN = m.topLN + lN

	// Body
	lX, lN = root.drawBody(lX, lN)

	// Section header
	root.drawSectionHeader()

	m.bottomLN = max(lN, 0)
	m.bottomLX = lX

	if root.mouseSelect {
		root.drawSelect(root.x1, root.y1, root.x2, root.y2, true)
	}

	root.drawStatus()
	root.Show()
}

// Prepare the contents.
func (root *Root) prepareContents(ctx context.Context) {
	m := root.Doc

	// clear contents
	for k := range root.scr.contents {
		delete(root.scr.contents, k)
	}
	// header
	root.setContents(m.SkipLines, m.firstLine())

	// sectionHeader
	sLN, err := root.searchSectionHeader(ctx)
	if err != nil {
		root.scr.sectionHeaderLN = -1

	} else {
		root.scr.sectionHeaderLN = sLN
		root.setContents(sLN, sLN+m.SectionHeaderNum)
	}

	m.sectionHeaderLen = 0
	// set the length of the section header.
	if !m.WrapMode {
		root.Doc.sectionHeaderLen = m.SectionHeaderNum
	} else {
		root.Doc.sectionHeaderLen = 0
		for i := sLN; i < sLN+m.SectionHeaderNum; i++ {
			root.Doc.sectionHeaderLen += len(m.leftMostX(i))
		}
	}

	if m.showGotoF && root.scr.sectionHeaderLN >= 0 && m.jumpTargetNum == 0 {
		if m.topLN >= sLN+m.SectionHeaderNum {
			m.moveYUp(root.Doc.sectionHeaderLen)
		}
	}
	m.showGotoF = false

	// body
	endLN := m.topLN + root.scr.vHeight // vHeight is the max line of logical lines.
	root.setContents(m.topLN, endLN)
}

// searchSectionHeader searches for the section header.
func (root *Root) searchSectionHeader(ctx context.Context) (int, error) {
	m := root.Doc
	lN := m.topLN + 1
	if !m.SectionHeader || m.SectionDelimiter == "" {
		return 0, ErrNoDelimiter
	}

	ctx, cancel := context.WithTimeout(ctx, sectionTimeOut*time.Millisecond)
	defer cancel()
	sLN, err := m.prevSection(ctx, lN)
	if err != nil {
		if errors.Is(err, ErrCancel) {
			root.setMessageLogf("Section header search timed out")
			m.SectionDelimiter = ""
		}
		return 0, ErrNoMoreSection
	}

	if m.Header > sLN {
		return 0, ErrNoMoreSection
	}

	return sLN, nil
}

// setContents sets the contents of the specified range.
func (root *Root) setContents(start int, end int) {
	m := root.Doc
	for lN := start; lN < end; lN++ {
		line := m.getLineC(lN, m.TabWidth)
		if line.valid {
			RangeStyle(line.lc, 0, len(line.lc), root.StyleBody)
			root.styleContent(line)
		}
		root.scr.contents[lN] = line
	}
}

// styleContent applies the style of the content.
func (root *Root) styleContent(line LineC) {
	if root.Doc.PlainMode {
		root.plainStyle(line.lc)
	}
	if root.Doc.ColumnMode {
		root.columnHighlight(line)
	}
	root.multiColorHighlight(line)
	root.searchHighlight(line)
}

// drawHeader draws header.
func (root *Root) drawHeader() int {
	m := root.Doc

	// lN is a logical line number.
	lN := m.SkipLines
	// lX is the logical x position of Contents.
	lX := 0

	// wrapNum is the number of wrapped lines.
	wrapNum := 0
	// y is the y-coordinate.
	y := 0
	for ; lN < m.firstLine(); y++ {
		if y > root.scr.vHeight {
			break
		}
		line := root.scr.contents[lN]

		root.scr.numbers[y] = newLineNumber(lN, wrapNum)

		nextLX, nextLN := root.drawLine(y, lX, lN, line.lc)
		if root.Doc.LineNumMode {
			root.blankLineNumber(y)
		}
		// header style
		root.yStyle(y, root.StyleHeader)

		wrapNum++
		if nextLX == 0 {
			wrapNum = 0
		}

		lX = nextLX
		lN = nextLN
	}
	m.headerLen = y
	return lN
}

// drawSectionHeader draws section header.
func (root *Root) drawSectionHeader() {
	m := root.Doc
	if !m.SectionHeader || m.SectionDelimiter == "" {
		return
	}

	if root.scr.sectionHeaderLN < 0 {
		return
	}

	sx := 0
	sn := root.scr.sectionHeaderLN
	wrapNum := 0
	for y := m.headerLen; sn < root.scr.sectionHeaderLN+m.SectionHeaderNum; y++ {
		line := root.scr.contents[sn]
		root.scr.numbers[y] = newLineNumber(sn, wrapNum)
		root.drawLineNumber(sn, y, line.valid)
		nextLX, nextLN := root.drawLine(y, sx, sn, line.lc)
		root.yStyle(y, root.StyleSectionLine)
		markStyleWidth := min(root.scr.vWidth, root.Doc.general.MarkStyleWidth)
		root.markStyle(sn, y, markStyleWidth)

		wrapNum++
		if nextLX == 0 {
			wrapNum = 0
		}

		sx = nextLX
		sn = nextLN
	}
}

// drawBody draws body.
func (root *Root) drawBody(lX int, lN int) (int, int) {
	m := root.Doc

	wrapNum := m.numOfWrap(lX, lN)
	for y := m.headerLen; y < root.scr.vHeight-statusLine; y++ {
		root.scr.numbers[y] = newLineNumber(lN, wrapNum)
		line := root.scr.contents[lN]
		nextLX, nextLN := root.drawLine(y, lX, lN, line.lc)

		root.drawLineNumber(lN, y, line.valid)
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

// coordinatesStyle applies the style of the coordinates.
func (root *Root) coordinatesStyle(lN int, y int, str string) {
	root.alternateRowsStyle(lN, y)
	root.sectionLineHighlight(y, str)
	markStyleWidth := min(root.scr.vWidth, root.Doc.general.MarkStyleWidth)
	root.markStyle(lN, y, markStyleWidth)
	if root.Doc.jumpTargetNum != 0 && root.Doc.headerLen+root.Doc.jumpTargetNum == y {
		root.yStyle(y, root.StyleJumpTargetLine)
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

// searchHighlight applies the style of the search highlight.
// Apply style to contents.
func (root *Root) searchHighlight(line LineC) {
	if root.searcher == nil || root.searcher.String() == "" {
		return
	}

	indexes := root.searchPosition(line.str)
	for _, idx := range indexes {
		RangeStyle(line.lc, line.pos.x(idx[0]), line.pos.x(idx[1]), root.StyleSearchHighlight)
	}
}

// plainStyle defaults to the original style.
func (*Root) plainStyle(lc contents) {
	for x := 0; x < len(lc); x++ {
		lc[x].style = tcell.StyleDefault
	}
}

// multiColorHighlight applies styles to multiple words (regular expressions) individually.
// The style of the first specified word takes precedence.
func (root *Root) multiColorHighlight(line LineC) {
	numC := len(root.StyleMultiColorHighlight)
	for i := len(root.Doc.multiColorRegexps) - 1; i >= 0; i-- {
		indexes := searchPositionReg(line.str, root.Doc.multiColorRegexps[i])
		for _, idx := range indexes {
			RangeStyle(line.lc, line.pos.x(idx[0]), line.pos.x(idx[1]), root.StyleMultiColorHighlight[i%numC])
		}
	}
}

// blankLineNumber should be blank for the line number.
func (root *Root) blankLineNumber(y int) {
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
		numC[i].style = applyStyle(tcell.StyleDefault, root.StyleLineNumber)
	}
	root.setContentString(0, y, numC)
}

// columnHighlight applies the style of the column highlight.
func (root *Root) columnHighlight(line LineC) {
	if root.Doc.ColumnWidth {
		root.columnWidthHighlight(line)
		return
	}
	root.columnDelimiterHighlight(line)
}

// columnHighlight applies the style of the column highlight.
func (root *Root) columnDelimiterHighlight(line LineC) {
	m := root.Doc
	indexes := allIndex(line.str, m.ColumnDelimiter, m.ColumnDelimiterReg)
	if len(indexes) == 0 {
		return
	}

	lStart := 0
	if indexes[0][0] == 0 {
		if len(indexes) == 1 {
			return
		}
		lStart = indexes[0][1]
		indexes = indexes[1:]
	}

	numC := len(root.StyleColumnRainbow)

	var iStart, iEnd int
	for c := 0; c < len(indexes)+1; c++ {
		switch {
		case c == 0 && lStart == 0:
			iStart = lStart
			iEnd = indexes[0][1] - len(m.ColumnDelimiter)
			if iEnd < 0 {
				iEnd = 0
			}
		case c < len(indexes):
			iStart = iEnd + 1
			iEnd = indexes[c][0]
		case c == len(indexes):
			iStart = iEnd + 1
			iEnd = len(line.str)
		}
		if iStart < 0 || iEnd < 0 {
			return
		}
		start, end := line.pos.x(iStart), line.pos.x(iEnd)
		if m.ColumnRainbow {
			RangeStyle(line.lc, start, end, root.StyleColumnRainbow[c%numC])
		}
		if c == m.columnCursor {
			RangeStyle(line.lc, start, end, root.StyleColumnHighlight)
		}
	}
}

func (root *Root) columnWidthHighlight(line LineC) {
	m := root.Doc
	indexes := m.columnWidths
	if len(indexes) == 0 {
		return
	}
	iStart, iEnd := 0, 0
	numC := len(root.StyleColumnRainbow)
	for c := 0; c < len(indexes)+1; c++ {
		switch {
		case c == 0:
			iStart = 0
			iEnd = findBounds(line.lc, indexes[0]-1, indexes, c)
		case c < len(indexes):
			iStart = iEnd + 1
			iEnd = findBounds(line.lc, indexes[c], indexes, c)
		case c == len(indexes):
			iStart = iEnd + 1
			iEnd = len(line.str)
		}
		iEnd = min(iEnd, len(line.lc))

		if m.ColumnRainbow {
			RangeStyle(line.lc, iStart, iEnd, root.StyleColumnRainbow[c%numC])
		}

		if c == m.columnCursor {
			RangeStyle(line.lc, iStart, iEnd, root.StyleColumnHighlight)
		}
	}
}

// findBounds finds the bounds of values that extend beyond the column position.
func findBounds(lc contents, p int, pos []int, n int) int {
	if len(lc) <= p {
		return p
	}
	if lc[p].mainc == ' ' {
		return p
	}
	f := p
	fp := 0
	for ; f < len(lc) && lc[f].mainc != ' '; f++ {
		fp++
	}

	b := p
	bp := 0
	for ; b > 0 && lc[b].mainc != ' '; b-- {
		bp++
	}

	if b == pos[n] {
		return f
	}
	if n < len(pos)-1 {
		if f == pos[n+1] {
			return b
		}
		if b == pos[n] {
			return f
		}
		if b > pos[n] && b < pos[n+1] {
			return b
		}
	}
	return f
}

// RangeStyle applies the style to the specified range.
// Apply style to contents.
func RangeStyle(lc contents, start int, end int, s OVStyle) {
	for x := start; x < end; x++ {
		lc[x].style = applyStyle(lc[x].style, s)
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
			root.SetContent(x, y, r, c, applyStyle(style, root.StyleMarkLine))
		}
	}
}

// drawStatus draws a status line.
func (root *Root) drawStatus() {
	root.clearY(root.Doc.statusPos)
	leftContents, cursorPos := root.leftStatus()
	root.setContentString(0, root.Doc.statusPos, leftContents)

	rightContents := root.rightStatus()
	root.setContentString(root.scr.vWidth-len(rightContents), root.Doc.statusPos, rightContents)

	root.Screen.ShowCursor(cursorPos, root.Doc.statusPos)
}

// leftStatus returns the status of the left side.
func (root *Root) leftStatus() (contents, int) {
	if root.input.Event.Mode() == Normal {
		return root.normalLeftStatus()
	}
	return root.inputLeftStatus()
}

// normalLeftStatus returns the status of the left side of the normal mode.
func (root *Root) normalLeftStatus() (contents, int) {
	var leftStatus strings.Builder
	if root.showDocNum && root.Doc.documentType != DocHelp && root.Doc.documentType != DocLog {
		leftStatus.WriteString("[")
		leftStatus.WriteString(strconv.Itoa(root.CurrentDoc))
		leftStatus.WriteString("]")
	}

	leftStatus.WriteString(root.statusDisplay())

	if root.Doc.Caption != "" {
		leftStatus.WriteString(root.Doc.Caption)
	} else if root.Config.Prompt.Normal.ShowFilename {
		leftStatus.WriteString(root.Doc.FileName)
	}
	leftStatus.WriteString(":")
	leftStatus.WriteString(root.message)
	leftContents := StrToContents(leftStatus.String(), -1)

	if root.Config.Prompt.Normal.InvertColor {
		color := tcell.ColorWhite
		if root.CurrentDoc != 0 {
			color = tcell.Color((root.CurrentDoc + 8) % 16)
		}
		for i := 0; i < len(leftContents); i++ {
			leftContents[i].style = leftContents[i].style.Foreground(tcell.ColorValid + color).Reverse(true)
		}
	}

	return leftContents, len(leftContents)
}

// statusDisplay returns the status mode of the document.
func (root *Root) statusDisplay() string {
	if root.Doc.WatchMode {
		// Watch mode doubles as FollowSection mode.
		return "(Watch)"
	}
	if root.Doc.FollowSection {
		return "(Follow Section)"
	}
	if root.General.FollowAll {
		return "(Follow All)"
	}
	if root.Doc.FollowMode && root.Doc.FollowName {
		return "(Follow Name)"
	}
	if root.Doc.FollowMode {
		return "(Follow Mode)"
	}
	return ""
}

// inputLeftStatus returns the status of the left side of the input.
func (root *Root) inputLeftStatus() (contents, int) {
	input := root.input
	prompt := root.inputPrompt()
	leftContents := StrToContents(prompt+input.value, -1)
	return leftContents, len(prompt) + input.cursorX
}

// inputPrompt returns a string describing the input field.
func (root *Root) inputPrompt() string {
	var prompt strings.Builder
	mode := root.input.Event.Mode()
	modePrompt := root.input.Event.Prompt()

	if mode == Search || mode == Backsearch || mode == Filter {
		prompt.WriteString(root.searchOpt)
	}
	prompt.WriteString(modePrompt)
	return prompt.String()
}

// rightStatus returns the status of the right side.
func (root *Root) rightStatus() contents {
	next := ""
	if !root.Doc.BufEOF() {
		next = "..."
	}
	str := fmt.Sprintf("(%d/%d%s)", root.Doc.topLN, root.Doc.BufEndNum(), next)
	if atomic.LoadInt32(&root.Doc.tmpFollow) == 1 {
		str = fmt.Sprintf("(?/%d%s)", root.Doc.storeEndNum(), next)
	}
	return StrToContents(str, -1)
}

// setContentString is a helper function that draws a string with setContent.
func (root *Root) setContentString(vx int, vy int, lc contents) {
	screen := root.Screen
	for x, content := range lc {
		screen.SetContent(vx+x, vy, content.mainc, content.combc, content.style)
	}
	screen.SetContent(vx+len(lc), vy, 0, nil, tcell.StyleDefault.Normal())
}

// clearEOL clears from the specified position to the right end.
func (root *Root) clearEOL(x int, y int) {
	for ; x < root.scr.vWidth; x++ {
		root.Screen.SetContent(x, y, ' ', nil, tcell.StyleDefault)
	}
}

// clearY clear the specified line.
func (root *Root) clearY(y int) {
	root.clearEOL(0, y)
}

// sectionLineHighlight applies the style of the section line highlight.
func (root *Root) sectionLineHighlight(y int, str string) {
	if root.Doc.SectionDelimiter == "" {
		return
	}

	if root.Doc.SectionDelimiterReg == nil {
		log.Printf("Regular expression is not set: %s", root.Doc.SectionDelimiter)
		return
	}
	root.scr.sectionHeaderLeft--
	if root.scr.sectionHeaderLeft > 0 {
		root.yStyle(y, root.StyleSectionLine)
	}
	if root.Doc.SectionDelimiterReg.MatchString(str) {
		root.yStyle(y, root.StyleSectionLine)
		root.scr.sectionHeaderLeft = root.Doc.SectionHeaderNum
	}
}
