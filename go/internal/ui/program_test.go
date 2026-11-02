package ui

import (
	tea "charm.land/bubbletea/v2"
	legacy "github.com/charmbracelet/bubbletea"
	"testing"
)

func TestHoldInstructionsDismissesOnlyOnReleaseOrFocusLoss(t *testing.T) {
	terminal := terminalModel{model: fixture(), releases: true}
	n, _ := terminal.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})
	terminal = n.(terminalModel)
	if !terminal.model.help || !terminal.model.helpHeld {
		t.Fatal("hold did not show shortcuts")
	}
	n, _ = terminal.Update(tea.KeyPressMsg{Code: 'i', Text: "i", IsRepeat: true})
	terminal = n.(terminalModel)
	if !terminal.model.help {
		t.Fatal("repeat closed shortcuts")
	}
	n, _ = terminal.Update(tea.KeyReleaseMsg{Code: 'i'})
	terminal = n.(terminalModel)
	if terminal.model.help || terminal.model.helpHeld {
		t.Fatal("shortcuts stayed open after release")
	}
	n, _ = terminal.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})
	terminal = n.(terminalModel)
	n, _ = terminal.Update(tea.BlurMsg{})
	terminal = n.(terminalModel)
	if terminal.model.help {
		t.Fatal("lost-focus hold got stuck")
	}
	terminal.model.inputMode = "search"
	n, _ = terminal.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})
	terminal = n.(terminalModel)
	if terminal.model.help || terminal.model.input != "i" {
		t.Fatal("search i opened shortcuts")
	}
}
func TestTerminalKeyTranslationKeepsEditingAndUppercaseShortcuts(t *testing.T) {
	for _, test := range []struct {
		input tea.Key
		want  string
	}{{tea.Key{Code: tea.KeyRight, Mod: tea.ModCtrl}, "ctrl+right"}, {tea.Key{Code: 'q', Text: "Q", Mod: tea.ModShift}, "Q"}, {tea.Key{Code: 'g', Mod: tea.ModCtrl}, "ctrl+g"}, {tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}, "shift+tab"}, {tea.Key{Code: tea.KeyLeft, Mod: tea.ModAlt}, "alt+left"}} {
		key, ok := legacyKey(test.input)
		if !ok || key.String() != test.want {
			t.Fatalf("%v translated to %q", test.input, key.String())
		}
	}
	if _, ok := terminalCommand(legacy.Quit)().(tea.QuitMsg); !ok {
		t.Fatal("quit was lost at runtime boundary")
	}
}
func TestGuideSectionClickAndNarrowOverlayStayWithinTerminal(t *testing.T) {
	for _, size := range [][2]int{{40, 15}, {80, 24}, {110, 30}, {230, 51}} {
		m := fixture()
		m.width = size[0]
		m.height = size[1]
		m.help = true
		doc, x, y := m.guideGeometry()
		g := doc.helpLayout()
		hit := g.hits[4]
		next, _ := m.clickHelp(x+1+hit.x, y+1+hit.y)
		m = next.(Model)
		if m.helpOffset == 0 {
			t.Fatal("section jump did not move")
		}
	}
}

func TestSingleCharacterPasteCannotExecuteShortcut(t *testing.T) {
	m := terminalModel{model: fixture()}
	next, cmd := m.Update(tea.PasteMsg{Content: "q"})
	if cmd != nil || next.(terminalModel).model.help {
		t.Fatal("paste executed a global shortcut")
	}
}
