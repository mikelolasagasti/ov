package oviewer

import (
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/gdamore/tcell/v2"
)

// statusLine is the number of lines in the status bar.
const statusLine = 1

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
