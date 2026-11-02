package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	guidePanel = lipgloss.NewStyle().Background(lipgloss.Color("#191B1A"))
	guideKey   = lipgloss.NewStyle().Foreground(lipgloss.Color("#D2EBDD")).Background(lipgloss.Color("#27382E")).Bold(true)
	guideText  = lipgloss.NewStyle().Foreground(lipgloss.Color("#C1C7C3")).Background(lipgloss.Color("#191B1A"))
	guideRule  = lipgloss.NewStyle().Foreground(lipgloss.Color("#343C36"))
)

type guideShortcut struct{ key, action string }
type guideSection struct {
	title, label string
	rows         []guideShortcut
	note         string
}

var guideSections = []guideSection{
	{title: "Playback", label: "Playback", rows: []guideShortcut{
		{"Space", "Pause or resume"},
		{"Ctrl+Right / n", "Next song"},
		{"Ctrl+Left / p", "Previous song; restart after 3s"},
		{"[ / ]\nAlt+Left/Right", "Seek backward / forward 5s"},
		{"+ / = / -", "Adjust volume by 5%"},
		{"s", "Toggle shuffle"},
		{"r", "Repeat: off / collection / song"},
		{"a", "Add selected song to the queue"},
		{"P", "Play playlist from the start"},
	}, note: "Ctrl-arrows work while you search, too."},
	{title: "Move around", label: "Navigation", rows: []guideShortcut{
		{"Up / Down\nj / k", "Move in the focused pane"},
		{"Left / h / Esc", "Return to your library"},
		{"Right / l\nEnter", "Open a collection; play a song"},
		{"Tab / Shift-Tab", "Switch library and songs"},
		{"PgUp / PgDn\nCtrl+D", "Page through the focused pane"},
		{"Home / g\nEnd / G", "Jump to the first / last item"},
	}},
	{title: "Find a song", label: "Search", rows: []guideShortcut{
		{"/ / Ctrl+F", "Search songs and artists"},
		{"Enter", "Run your search"},
		{"Up / Down", "Recall a previous search"},
		{"Esc", "Cancel; keep your collection"},
		{"Ctrl+G", "Clear the search input"},
		{"Ctrl+X", "Clear saved search history"},
	}, note: "Click any recent search to run it again."},
	{title: "Edit your search", label: "Editing", rows: []guideShortcut{
		{"Left / Right\nCtrl+B / Ctrl+F", "Move the cursor"},
		{"Home / Ctrl+A", "Jump to the start"},
		{"End / Ctrl+E", "Jump to the end"},
		{"Backspace\nDelete / Ctrl+D", "Erase before / after the cursor"},
		{"Ctrl+W", "Erase the previous word"},
		{"Ctrl+K", "Erase to the end"},
	}},
	{title: "Your app", label: "App", rows: []guideShortcut{
		{"L", "Sign in through your browser"},
		{"c", "Reconnect the local player"},
		{"R / Ctrl+R", "Refresh the focused pane"},
		{"Ctrl+U", "Install a stable update; restart"},
		{"Hold i", "Show shortcuts until released"},
		{"? / F1", "Toggle the shortcut overlay"},
		{"q / Ctrl+C", "Close terminal; music continues"},
		{"Q", "Stop audio, tray and media controls; quit"},
		{"d / D", "Show offline download status"},
	}},
	{title: "Mouse & desktop", label: "Mouse", rows: []guideShortcut{
		{"Click collection", "Open that collection"},
		{"Click song", "Play the selected song"},
		{"Mouse wheel", "Scroll the pane under the pointer"},
		{"Click progress", "Seek to that point in the song"},
		{"Click volume", "Adjust the volume"},
		{"Player buttons", "Previous, play/pause, next, shuffle, repeat"},
		{"Shift + select", "Use your terminal's text selection and copy"},
		{"Media keys\nplayerctl", "Linux MPRIS controls while the local player runs"},
	}},
}

type guideHit struct{ x, y, width, section int }
type guideLayout struct {
	width, left, bodyHeight int
	header, body            []string
	anchors                 []int
	hits                    []guideHit
	backX, backWidth        int
}

func guideWrap(text string, width int) []string {
	return strings.Split(ansi.Wrap(text, max(1, width), ""), "\n")
}
func guideBadge(text string) string { return guideKey.Render(" " + text + " ") }
func guideCard(section guideSection, number, width int) []string {
	inner := max(1, width-4)
	lines := []string{"", "  " + active.Background(lipgloss.Color("#191B1A")).Render(fmt.Sprintf("%02d", number)) + "  " + bright.Bold(true).Background(lipgloss.Color("#191B1A")).Render(section.title), "  " + guideRule.Render(strings.Repeat("─", inner))}
	keyW := min(22, max(12, inner*2/5))
	actionW := max(1, inner-keyW-2)
	for _, row := range section.rows {
		keys := guideWrap(row.key, keyW-2)
		actions := guideWrap(row.action, actionW)
		for i := 0; i < max(len(keys), len(actions)); i++ {
			label, action := "", ""
			if i < len(keys) {
				label = guideBadge(keys[i])
			}
			if i < len(actions) {
				action = guideText.Render(actions[i])
			}
			lines = append(lines, "  "+fit(label, keyW)+"  "+action)
		}
	}
	if section.note != "" {
		lines = append(lines, "")
		for _, line := range guideWrap(section.note, inner) {
			lines = append(lines, "  "+muted.Background(lipgloss.Color("#191B1A")).Render(line))
		}
	}
	lines = append(lines, "")
	for i := range lines {
		lines[i] = guidePanel.Render(fit(lines[i], width))
	}
	return lines
}
func (m Model) helpLayout() guideLayout {
	g := guideLayout{width: min(124, max(1, m.width-4))}
	g.left = max(0, (m.width-g.width)/2)
	back := guideBadge("Esc") + muted.Render(" Back to music")
	if m.helpHeld {
		back = guideBadge("i") + muted.Render(" Release to close")
	}
	brand := active.Render("spotify tui") + muted.Render(" / INSTRUCTIONS")
	g.backWidth = lipgloss.Width(back)
	g.backX = g.left + g.width - g.backWidth
	top := brand
	if lipgloss.Width(brand)+g.backWidth+3 <= g.width {
		top = brand + strings.Repeat(" ", g.width-lipgloss.Width(brand)-g.backWidth) + back
	} else {
		g.backWidth = 0
	}
	g.header = []string{"", top, ""}
	if m.width >= 70 && m.height >= 22 {
		g.header = append(g.header, bright.Bold(true).Render("Your shortcuts, at a glance."), muted.Render("Hold i to peek. Release it to get back to your music."), "")
	}
	nav := ""
	for i, section := range guideSections {
		item := guideBadge(fmt.Sprint(i+1)) + " " + muted.Render(section.label)
		itemW := lipgloss.Width(item)
		gap := ""
		if nav != "" {
			gap = "   "
		}
		if lipgloss.Width(nav)+len(gap)+itemW > g.width && nav != "" {
			g.header = append(g.header, nav)
			nav = ""
			gap = ""
		}
		g.hits = append(g.hits, guideHit{x: g.left + lipgloss.Width(nav) + len(gap), y: len(g.header), width: itemW, section: i})
		nav += gap + item
	}
	g.header = append(g.header, nav, "")
	g.anchors = make([]int, len(guideSections))
	columns := 1
	if g.width >= 100 {
		columns = 2
	}
	cardW := g.width - 2 // Leave a gutter for the scroll indicator.
	if columns == 2 {
		cardW = (g.width - 4) / 2
	}
	for i := 0; i < len(guideSections); i += columns {
		g.anchors[i] = len(g.body)
		left := guideCard(guideSections[i], i+1, cardW)
		if columns == 1 {
			g.body = append(g.body, left...)
		} else {
			g.anchors[i+1] = len(g.body)
			right := guideCard(guideSections[i+1], i+2, cardW)
			blank := guidePanel.Render(strings.Repeat(" ", cardW))
			for line := 0; line < max(len(left), len(right)); line++ {
				a, b := blank, blank
				if line < len(left) {
					a = left[line]
				}
				if line < len(right) {
					b = right[line]
				}
				g.body = append(g.body, a+"  "+b)
			}
		}
		g.body = append(g.body, "")
	}
	g.body = append(g.body, active.Render("  Start with Explore"))
	for _, note := range []string{
		"Pick a familiar song in Explore, then Spotify Autoplay can suggest what comes next. Enable Autoplay in your Spotify account settings; availability depends on Spotify.",
		"Liked songs and playlists continue from your selection. Search and Explore start a Spotify Autoplay seed.",
		"Your history stays on this computer: 30 searches and 100 played songs. Demo history stays in memory; logout removes local history.",
	} {
		for _, line := range guideWrap(note, g.width-6) {
			g.body = append(g.body, "  "+muted.Background(lipgloss.Color("#191B1A")).Render(line))
		}
		g.body = append(g.body, "")
	}
	for _, line := range guideWrap("Offline Spotify downloads are currently unsupported.", g.width-6) {
		g.body = append(g.body, "  "+muted.Render(line))
	}
	g.bodyHeight = max(1, m.height-len(g.header)-3)
	return g
}
func (m Model) guideGeometry() (Model, int, int) {
	width, height := min(128, m.width-4), min(42, m.height-4)
	if m.width < 60 || m.height < 20 {
		width, height = m.width, m.height
	}
	doc := m
	doc.width = max(1, width-2)
	doc.height = max(1, height-2)
	return doc, (m.width - width) / 2, (m.height - height) / 2
}
func (m Model) helpLines() []string { doc, _, _ := m.guideGeometry(); return doc.helpLayout().body }
func (m *Model) scrollHelp(delta int) {
	doc, _, _ := m.guideGeometry()
	g := doc.helpLayout()
	m.helpOffset = max(0, min(m.helpOffset+delta, max(0, len(g.body)-g.bodyHeight)))
}
func (m Model) helpDocumentView() string {
	g := m.helpLayout()
	offset := max(0, min(m.helpOffset, max(0, len(g.body)-g.bodyHeight)))
	lines := append([]string(nil), g.header...)
	scrollTop, scrollSize := 0, g.bodyHeight
	if len(g.body) > g.bodyHeight {
		scrollSize = max(1, g.bodyHeight*g.bodyHeight/len(g.body))
		scrollTop = offset * (g.bodyHeight - scrollSize) / max(1, len(g.body)-g.bodyHeight)
	}
	for row := 0; row < g.bodyHeight; row++ {
		content := ""
		if offset+row < len(g.body) {
			content = g.body[offset+row]
		}
		rail := " "
		if len(g.body) > g.bodyHeight {
			rail = guideRule.Render("│")
			if row >= scrollTop && row < scrollTop+scrollSize {
				rail = active.Render("┃")
			}
		}
		lines = append(lines, fit(content, g.width-1)+rail)
	}
	progress := 100
	if len(g.body) > g.bodyHeight {
		progress = (offset + g.bodyHeight) * 100 / len(g.body)
	}
	footer := muted.Render("Up/Down scroll · PgUp/PgDn page · Home/End jump")
	if g.width < 70 {
		footer = muted.Render("Up/Down scroll · Esc back")
	}
	end := active.Render(fmt.Sprintf("%d%%", progress)) + muted.Render("  ") + guideBadge("Esc") + muted.Render(" Back")
	if lipgloss.Width(footer)+lipgloss.Width(end)+2 > g.width {
		footer = muted.Render("Scroll")
	}
	lines = append(lines, guideRule.Render(strings.Repeat("─", g.width)), footer+strings.Repeat(" ", max(1, g.width-lipgloss.Width(footer)-lipgloss.Width(end)))+end, "")
	for i := range lines {
		lines[i] = strings.Repeat(" ", g.left) + lines[i]
	}
	return bounded(lines, m.width, m.height)
}
func (m Model) helpView() string {
	doc, left, top := m.guideGeometry()
	base := m
	base.help = false
	behind := strings.Split(base.View(), "\n")
	sheet := strings.Split(lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#36503F")).Render(doc.helpDocumentView()), "\n")
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#59625C")).Background(lipgloss.Color("#121212"))
	for y := 0; y < m.height; y++ {
		line := ""
		if y < len(behind) {
			line = ansi.Strip(behind[y])
		} else {
			behind = append(behind, "")
		}
		if y >= top && y < top+len(sheet) {
			overlay := sheet[y-top]
			right := left + lipgloss.Width(overlay)
			behind[y] = dim.Render(fit(ansi.Cut(line, 0, left), left)) + overlay + dim.Render(fit(ansi.Cut(line, right, m.width), max(0, m.width-right)))
		} else {
			behind[y] = dim.Render(fit(line, m.width))
		}
	}
	return strings.Join(behind[:m.height], "\n")
}
func (m Model) clickHelp(x, y int) (tea.Model, tea.Cmd) {
	doc, left, top := m.guideGeometry()
	x -= left + 1
	y -= top + 1
	g := doc.helpLayout()
	for _, hit := range g.hits {
		if y == hit.y && x >= hit.x && x < hit.x+hit.width {
			m.helpOffset = g.anchors[hit.section]
			m.scrollHelp(0)
			return m, nil
		}
	}
	if y == 1 && g.backWidth > 0 && x >= g.backX && x < g.backX+g.backWidth || y == doc.height-2 && x >= g.left+g.width-10 {
		m.help = false
	}
	return m, nil
}
func (m Model) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	doc, _, _ := m.guideGeometry()
	g := doc.helpLayout()
	switch msg.String() {
	case "esc", "i", "?", "f1":
		m.help = false
	case "up", "k":
		m.scrollHelp(-1)
	case "down", "j":
		m.scrollHelp(1)
	case "pgdown", "ctrl+d", " ":
		m.scrollHelp(max(1, g.bodyHeight-1))
	case "pgup":
		m.scrollHelp(-max(1, g.bodyHeight-1))
	case "home", "g":
		m.helpOffset = 0
	case "end", "G":
		m.helpOffset = len(g.body)
		m.scrollHelp(0)
	case "1", "2", "3", "4", "5", "6":
		m.helpOffset = g.anchors[int(msg.String()[0]-'1')]
		m.scrollHelp(0)
	case "Q":
		m.help = false
		return m.updateKey(msg)
	case "q":
		m.cancelWork()
		return m, tea.Quit
	}
	return m, nil
}
