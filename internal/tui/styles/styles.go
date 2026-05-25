package styles

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	lipglossv2 "charm.land/lipgloss/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/rivo/uniseg"
)

var (
	Background lipgloss.Color
	Foreground lipgloss.Color
	Accent     lipgloss.Color
	Success    lipgloss.Color
	Warning    lipgloss.Color
	Error      lipgloss.Color
	Info       lipgloss.Color
	Dim        lipgloss.Color
	Separator  lipgloss.Color
	Surface    lipgloss.Color

	WorkingGradFromColor color.Color
	WorkingGradToColor   color.Color

	AppStyle lipgloss.Style

	WelcomeBorder lipgloss.Style
	WelcomeTitle  lipgloss.Style
	WelcomeText   lipgloss.Style

	UserPrefix  lipgloss.Style
	AgentPrefix lipgloss.Style
	MessageRole lipgloss.Style
	UserBubble  lipgloss.Style
	AgentBubble lipgloss.Style
	ErrorBubble lipgloss.Style

	ToolExecuting lipgloss.Style
	ToolName      lipgloss.Style
	ToolSuccess   lipgloss.Style
	ToolError     lipgloss.Style
	ToolOutput    lipgloss.Style

	TextareaStyle  textarea.Styles
	InputSeparator lipgloss.Style
	InputPrompt    lipgloss.Style
	InputBorder    lipgloss.Style
	InputLine      lipgloss.Style
	CursorStyle    lipgloss.Style

	DialogBorder   lipgloss.Style
	DialogTitle    lipgloss.Style
	DialogText     lipgloss.Style
	DialogMuted    lipgloss.Style
	DialogSelected lipgloss.Style
	DialogLineBase lipgloss.Style
	DialogGradFrom color.Color
	DialogGradTo   color.Color
	DialogView     lipgloss.Style
)

func init() {
	base := lipgloss.NewStyle().Foreground(toLipColor(charmtone.Ash))
	muted := lipgloss.NewStyle().Foreground(toLipColor(charmtone.Squid))
	subtle := lipgloss.NewStyle().Foreground(toLipColor(charmtone.Oyster))
	baseV2 := lipglossv2.NewStyle().Foreground(toLipColorV2(charmtone.Ash))
	mutedV2 := lipglossv2.NewStyle().Foreground(toLipColorV2(charmtone.Squid))
	subtleV2 := lipglossv2.NewStyle().Foreground(toLipColorV2(charmtone.Oyster))

	Background = toLipColor(charmtone.Pepper)
	Foreground = toLipColor(charmtone.Ash)
	Accent = toLipColor(charmtone.Bok)
	Success = toLipColor(charmtone.Julep)
	Warning = toLipColor(charmtone.Mustard)
	Error = toLipColor(charmtone.Sriracha)
	Info = toLipColor(charmtone.Malibu)
	Dim = toLipColor(charmtone.Squid)
	Separator = toLipColor(charmtone.Charcoal)
	Surface = toLipColor(charmtone.BBQ)

	WorkingGradFromColor = charmtone.Charple
	WorkingGradToColor = charmtone.Dolly

	AppStyle = lipgloss.NewStyle().Background(toLipColor(charmtone.Pepper)).Foreground(toLipColor(charmtone.Ash))

	WelcomeBorder = base.Padding(1, 2)
	WelcomeTitle = base.Foreground(toLipColor(charmtone.Bok)).Bold(true)
	WelcomeText = muted

	UserPrefix = base.Foreground(toLipColor(charmtone.Bok)).Bold(true)
	AgentPrefix = base.Foreground(toLipColor(charmtone.Malibu)).Bold(true)
	MessageRole = muted.Bold(true)
	UserBubble = lipgloss.NewStyle().
		Foreground(toLipColor(charmtone.Charcoal)).
		Background(toLipColor(charmtone.Ash)).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(toLipColor(charmtone.Smoke)).
		Padding(0, 1)
	AgentBubble = base.
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(toLipColor(charmtone.Bok)).
		PaddingLeft(1)
	ErrorBubble = base.Foreground(toLipColor(charmtone.Sriracha)).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(toLipColor(charmtone.Sriracha)).
		PaddingLeft(1)

	ToolExecuting = base.Foreground(toLipColor(charmtone.Citron))
	ToolName = base.Foreground(toLipColor(charmtone.Malibu)).Bold(true)
	ToolSuccess = base.Foreground(toLipColor(charmtone.Julep))
	ToolError = base.Foreground(toLipColor(charmtone.Sriracha))
	ToolOutput = muted

	TextareaStyle = textarea.Styles{
		Focused: textarea.StyleState{
			Base:             baseV2,
			Text:             baseV2,
			LineNumber:       subtleV2,
			CursorLine:       baseV2,
			CursorLineNumber: subtleV2,
			Placeholder:      subtleV2,
			Prompt:           baseV2.Foreground(toLipColorV2(charmtone.Bok)),
		},
		Blurred: textarea.StyleState{
			Base:             mutedV2,
			Text:             mutedV2,
			LineNumber:       subtleV2,
			CursorLine:       mutedV2,
			CursorLineNumber: subtleV2,
			Placeholder:      subtleV2,
			Prompt:           mutedV2,
		},
		Cursor: textarea.CursorStyle{
			Color: toLipColorV2(charmtone.Dolly),
			Shape: tea.CursorBar,
			Blink: true,
		},
	}
	InputSeparator = subtle
	InputPrompt = UserPrefix
	InputBorder = subtle
	InputLine = base
	CursorStyle = base.Foreground(toLipColor(charmtone.Dolly))

	DialogBorder = subtle
	DialogTitle = base.Foreground(toLipColor(charmtone.Bok)).Bold(true)
	DialogText = base
	DialogMuted = muted
	DialogSelected = lipgloss.NewStyle().
		Foreground(toLipColor(charmtone.Butter)).
		Background(toLipColor(charmtone.Charple)).
		Bold(true)
	DialogLineBase = lipgloss.NewStyle()
	DialogGradFrom = charmtone.Charple
	DialogGradTo = charmtone.Dolly
	DialogView = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(toLipColor(charmtone.Charcoal))
}

func ForegroundGrad(base lipgloss.Style, input string, bold bool, color1, color2 color.Color) []string {
	if input == "" {
		return []string{""}
	}
	var clusters []string
	gr := uniseg.NewGraphemes(input)
	for gr.Next() {
		clusters = append(clusters, string(gr.Runes()))
	}
	if len(clusters) == 0 {
		return []string{""}
	}
	for i, cluster := range clusters {
		t := 0.0
		if len(clusters) > 1 {
			t = float64(i) / float64(len(clusters)-1)
		}
		style := base.Foreground(toLipColor(blend(color1, color2, t)))
		if bold {
			style = style.Bold(true)
		}
		clusters[i] = style.Render(cluster)
	}
	return clusters
}

func ApplyForegroundGrad(base lipgloss.Style, input string, color1, color2 color.Color) string {
	if input == "" {
		return ""
	}
	var b strings.Builder
	for _, part := range ForegroundGrad(base, input, false, color1, color2) {
		fmt.Fprint(&b, part)
	}
	return b.String()
}

func ApplyBoldForegroundGrad(base lipgloss.Style, input string, color1, color2 color.Color) string {
	if input == "" {
		return ""
	}
	var b strings.Builder
	for _, part := range ForegroundGrad(base, input, true, color1, color2) {
		fmt.Fprint(&b, part)
	}
	return b.String()
}

func blend(a, b color.Color, t float64) color.Color {
	ca, _ := colorful.MakeColor(a)
	cb, _ := colorful.MakeColor(b)
	return ca.BlendLab(cb, t)
}

func toLipColor(c color.Color) lipgloss.Color {
	r, g, b, _ := c.RGBA()
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", uint8(r>>8), uint8(g>>8), uint8(b>>8)))
}

func toLipColorV2(c color.Color) color.Color {
	r, g, b, _ := c.RGBA()
	return lipglossv2.Color(fmt.Sprintf("#%02x%02x%02x", uint8(r>>8), uint8(g>>8), uint8(b>>8)))
}
