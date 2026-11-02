package ui

import (
	"unicode"

	tea "charm.land/bubbletea/v2"
	legacy "github.com/charmbracelet/bubbletea"
)

// The native input runtime handles terminal key releases; the application keeps
// its existing command/model API so playback and update behavior stay shared.
type terminalModel struct {
	model    Model
	releases bool
}

func Run(model Model) (Model, error) {
	result, err := tea.NewProgram(terminalModel{model: model}).Run()
	if err != nil {
		return model, err
	}
	return result.(terminalModel).model, nil
}
func terminalCommand(cmd legacy.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		switch value := msg.(type) {
		case legacy.QuitMsg:
			return tea.Quit()
		case legacy.BatchMsg:
			commands := make([]tea.Cmd, 0, len(value))
			for _, c := range value {
				commands = append(commands, terminalCommand(c))
			}
			return tea.Batch(commands...)()
		default:
			return msg
		}
	}
}
func (t terminalModel) Init() tea.Cmd { return terminalCommand(t.model.Init()) }
func (t terminalModel) View() tea.View {
	v := tea.NewView(t.model.View())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.ReportFocus = true
	v.KeyboardEnhancements.ReportEventTypes = true
	v.KeyboardEnhancements.ReportAlternateKeys = true
	v.KeyboardEnhancements.ReportAssociatedText = true
	v.KeyboardEnhancements.ReportAllKeysAsEscapeCodes = true
	return v
}
func (t terminalModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var event legacy.Msg = msg
	switch value := msg.(type) {
	case tea.KeyboardEnhancementsMsg:
		t.releases = value.SupportsEventTypes()
		return t, nil
	case tea.BlurMsg:
		if t.model.helpHeld {
			t.model.help = false
			t.model.helpHeld = false
		}
		return t, nil
	case tea.KeyReleaseMsg:
		if value.Code == 'i' && t.model.helpHeld {
			t.model.help = false
			t.model.helpHeld = false
		}
		return t, nil
	case tea.KeyPressMsg:
		if value.Code == 'i' && value.Mod&(tea.ModCtrl|tea.ModAlt|tea.ModShift|tea.ModSuper) == 0 && t.model.inputMode == "" {
			if value.IsRepeat {
				return t, nil
			}
			if t.releases {
				t.model.help = true
				t.model.helpHeld = true
				t.model.helpOffset = 0
			} else {
				t.model.notify("Hold i needs key-release support (Kitty, Ghostty or Foot). ? opens shortcuts.", "info")
			}
			return t, nil
		}
		key, ok := legacyKey(value.Key())
		if !ok {
			return t, nil
		}
		event = key
	case tea.PasteMsg:
		if t.model.inputMode == "" {
			t.model.notify("Press / before pasting a search.", "info")
			return t, nil
		}
		event = legacy.KeyMsg{Type: legacy.KeyRunes, Runes: []rune(value.Content), Paste: true}
	case tea.WindowSizeMsg:
		event = legacy.WindowSizeMsg{Width: value.Width, Height: value.Height}
	case tea.MouseMsg:
		mouse := value.Mouse()
		button := legacy.MouseButtonNone
		switch mouse.Button {
		case tea.MouseLeft:
			button = legacy.MouseButtonLeft
		case tea.MouseMiddle:
			button = legacy.MouseButtonMiddle
		case tea.MouseRight:
			button = legacy.MouseButtonRight
		case tea.MouseWheelUp:
			button = legacy.MouseButtonWheelUp
		case tea.MouseWheelDown:
			button = legacy.MouseButtonWheelDown
		}
		action := legacy.MouseActionPress
		switch msg.(type) {
		case tea.MouseReleaseMsg:
			action = legacy.MouseActionRelease
		case tea.MouseMotionMsg:
			action = legacy.MouseActionMotion
		}
		event = legacy.MouseMsg{X: mouse.X, Y: mouse.Y, Button: button, Action: action, Ctrl: mouse.Mod&tea.ModCtrl != 0, Alt: mouse.Mod&tea.ModAlt != 0, Shift: mouse.Mod&tea.ModShift != 0}
	}
	next, cmd := t.model.Update(event)
	t.model = next.(Model)
	return t, terminalCommand(cmd)
}
func legacyKey(key tea.Key) (legacy.KeyMsg, bool) {
	name := key.Keystroke()
	if len(name) == 6 && name[:5] == "ctrl+" && name[5] >= 'a' && name[5] <= 'z' {
		return legacy.KeyMsg{Type: legacy.KeyType(name[5] - 'a' + 1)}, true
	}
	keys := map[string]legacy.KeyType{
		"enter": legacy.KeyEnter, "esc": legacy.KeyEsc, "escape": legacy.KeyEsc, "tab": legacy.KeyTab, "shift+tab": legacy.KeyShiftTab, "backspace": legacy.KeyBackspace, "delete": legacy.KeyDelete,
		"up": legacy.KeyUp, "down": legacy.KeyDown, "left": legacy.KeyLeft, "right": legacy.KeyRight, "home": legacy.KeyHome, "end": legacy.KeyEnd, "pgup": legacy.KeyPgUp, "pgdown": legacy.KeyPgDown,
		"ctrl+left": legacy.KeyCtrlLeft, "ctrl+right": legacy.KeyCtrlRight, "f1": legacy.KeyF1, "space": legacy.KeySpace,
	}
	if value, ok := keys[name]; ok {
		return legacy.KeyMsg{Type: value}, true
	}
	if name == "alt+left" {
		return legacy.KeyMsg{Type: legacy.KeyLeft, Alt: true}, true
	}
	if name == "alt+right" {
		return legacy.KeyMsg{Type: legacy.KeyRight, Alt: true}, true
	}
	if key.Text != "" {
		return legacy.KeyMsg{Type: legacy.KeyRunes, Runes: []rune(key.Text), Alt: key.Mod&tea.ModAlt != 0}, true
	}
	if key.Mod&(tea.ModCtrl|tea.ModAlt|tea.ModSuper) == 0 && unicode.IsPrint(key.Code) {
		r := key.Code
		if key.Mod&tea.ModShift != 0 {
			if key.ShiftedCode != 0 {
				r = key.ShiftedCode
			} else {
				r = unicode.ToUpper(r)
			}
		}
		return legacy.KeyMsg{Type: legacy.KeyRunes, Runes: []rune{r}}, true
	}
	return legacy.KeyMsg{}, false
}
