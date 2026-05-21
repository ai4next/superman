package styles

import "github.com/charmbracelet/lipgloss"

var (
	Background = lipgloss.Color("#111827")
	Foreground = lipgloss.Color("#d1d5db")
	Accent     = lipgloss.Color("#67e8f9")
	Success    = lipgloss.Color("#86efac")
	Warning    = lipgloss.Color("#fbbf24")
	Error      = lipgloss.Color("#fca5a5")
	Info       = lipgloss.Color("#93c5fd")
	Dim        = lipgloss.Color("#666666")
	Separator  = lipgloss.Color("#4d4d4d")
)

var AppStyle = lipgloss.NewStyle().
	Background(Background).
	Foreground(Foreground)

var WelcomeBorder = lipgloss.NewStyle().
	Border(lipgloss.DoubleBorder()).
	BorderForeground(lipgloss.Color("dodger_blue1")).
	Padding(1, 2)

var WelcomeTitle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("dodger_blue1")).
	Bold(true)

var WelcomeText = lipgloss.NewStyle().
	Foreground(Dim)

var UserPrefix = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#67e8f9")).
	Bold(true)

var AgentPrefix = lipgloss.NewStyle().
	Foreground(Accent).
	Bold(true)

var ToolExecuting = lipgloss.NewStyle().
	Foreground(Info)

var ToolSuccess = lipgloss.NewStyle().
	Foreground(Success)

var ToolError = lipgloss.NewStyle().
	Foreground(Error)

var ToolOutput = lipgloss.NewStyle().
	Foreground(Dim)

var InputSeparator = lipgloss.NewStyle().
	Foreground(Separator)

var InputPrompt = lipgloss.NewStyle().
	Foreground(Warning).
	Bold(true)

var CursorStyle = lipgloss.NewStyle().
	Foreground(Accent)

var ToolbarStyle = lipgloss.NewStyle().
	Foreground(Dim)

var SpinnerStyle = lipgloss.NewStyle().
	Foreground(Accent)