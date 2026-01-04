package tui

import "github.com/charmbracelet/lipgloss"

// Colors
var (
	colorPrimary   = lipgloss.Color("#7D56F4")
	colorWhite     = lipgloss.Color("#FAFAFA")
	colorGray      = lipgloss.Color("#888888")
	colorGreen     = lipgloss.Color("#00FF00")
	colorGold      = lipgloss.Color("#FFD700")
	colorDimGray   = lipgloss.Color("#555555")
	colorBorder    = lipgloss.Color("#444444")
	colorHighlight = lipgloss.Color("#3D3D3D")
)

// Title styles
var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Background(colorPrimary).
			Padding(0, 1)
)

// Pane border styles
var (
	PaneBorderFocused = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPrimary)

	PaneBorderUnfocused = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorder)
)

// Workspace list item styles
var (
	WorkspaceItemNormal = lipgloss.NewStyle().
				Foreground(colorWhite)

	WorkspaceItemSelected = lipgloss.NewStyle().
				Foreground(colorWhite).
				Background(colorHighlight).
				Bold(true)
)

// Agent state indicators
const (
	IndicatorDefault = "●" // Green - default workspace
	IndicatorRunning = "◐" // Gold - agent running
	IndicatorIdle    = "○" // Gray - agent idle
)

var (
	IndicatorDefaultStyle = lipgloss.NewStyle().
				Foreground(colorGreen)

	IndicatorRunningStyle = lipgloss.NewStyle().
				Foreground(colorGold)

	IndicatorIdleStyle = lipgloss.NewStyle().
				Foreground(colorDimGray)
)

// Help bar style
var HelpStyle = lipgloss.NewStyle().
	Foreground(colorGray)

// Error style
var ErrorStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FF0000")).
	Bold(true)

// Empty diff style
var EmptyDiffStyle = lipgloss.NewStyle().
	Foreground(colorGray).
	Italic(true)

// Confirm dialog styles
var (
	ConfirmBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(1, 2).
			Background(lipgloss.Color("#1A1A1A"))

	ConfirmPromptStyle = lipgloss.NewStyle().
				Foreground(colorWhite).
				Bold(true)
)

// Tab bar styles
var (
	TabBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1A1A1A"))

	TabActiveStyle = lipgloss.NewStyle().
			Foreground(colorWhite).
			Background(colorPrimary).
			Bold(true).
			Padding(0, 1)

	TabInactiveStyle = lipgloss.NewStyle().
				Foreground(colorGray).
				Background(lipgloss.Color("#2A2A2A")).
				Padding(0, 1)
)

// Chat message styles
var (
	ChatUserStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	ChatAgentStyle = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	ChatToolStyle = lipgloss.NewStyle().
			Foreground(colorGray)

	ChatToolSuccessStyle = lipgloss.NewStyle().
				Foreground(colorGreen)

	ChatToolErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF0000"))

	ChatModeNormalStyle = lipgloss.NewStyle().
				Foreground(colorGray)

	ChatModeInsertStyle = lipgloss.NewStyle().
				Foreground(colorGold).
				Bold(true)
)
