package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	supermansession "github.com/ai4next/superman/internal/session"
	"github.com/ai4next/superman/internal/tui/styles"
)

type SessionDialogData struct {
	Sessions []supermansession.Metadata
	Selected int
	Current  string
}

type CommandDialogItem struct {
	ID          string
	Title       string
	Description string
	Key         string
}

type CommandDialogData struct {
	Commands []CommandDialogItem
	Selected int
	Query    string
}

type FilePickerData struct {
	Files    []string
	Selected int
	Query    string
	CWD      string
}

type SearchResultsData struct {
	Results  []supermansession.MessageSearchResult
	Selected int
	Query    string
}

func RenderFilePicker(data FilePickerData, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	dialogWidth := min(max(52, width/2), max(20, width-4))
	inner := max(10, dialogWidth-4)
	maxItems := max(1, min(len(data.Files), height-8))
	start := 0
	if data.Selected >= maxItems {
		start = data.Selected - maxItems + 1
	}
	end := min(len(data.Files), start+maxItems)

	query := data.Query
	if query == "" {
		query = " "
	}
	var lines []string
	lines = append(lines, styles.DialogTitle.Render("Files"))
	lines = append(lines, styles.DialogMuted.Render("Type filter  Enter insert  Esc close  Up/Down move"))
	lines = append(lines, styles.DialogText.Render("filter: "+query))
	if data.CWD != "" {
		lines = append(lines, styles.DialogMuted.Render(TruncateRunes(data.CWD, inner)))
	}
	lines = append(lines, "")
	if len(data.Files) == 0 {
		lines = append(lines, styles.DialogMuted.Render("No files found"))
	} else {
		for i := start; i < end; i++ {
			cursor := " "
			if i == data.Selected {
				cursor = styles.CursorIcon
			}
			line := cursor + " " + data.Files[i]
			if i == data.Selected {
				lines = append(lines, styles.DialogSelected.Render(TruncateRunes(line, inner)))
			} else {
				lines = append(lines, styles.DialogText.Render(TruncateRunes(line, inner)))
			}
		}
		if end < len(data.Files) {
			lines = append(lines, styles.DialogMuted.Render(fmt.Sprintf("... %d more", len(data.Files)-end)))
		}
	}
	return renderDialogBox(lines, width, height, inner)
}

func RenderCommandDialog(data CommandDialogData, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	dialogWidth := min(max(48, width/2), max(20, width-4))
	inner := max(10, dialogWidth-4)
	maxItems := max(1, min(len(data.Commands), height-8))
	start := 0
	if data.Selected >= maxItems {
		start = data.Selected - maxItems + 1
	}
	end := min(len(data.Commands), start+maxItems)

	var lines []string
	lines = append(lines, styles.DialogTitle.Render("Commands"))
	lines = append(lines, styles.DialogMuted.Render("Type filter  Enter run  Esc close  Up/Down move"))
	query := data.Query
	if query == "" {
		query = " "
	}
	lines = append(lines, styles.DialogText.Render("filter: "+query))
	lines = append(lines, "")
	if len(data.Commands) == 0 {
		lines = append(lines, styles.DialogMuted.Render("No commands available"))
	} else {
		for i := start; i < end; i++ {
			command := data.Commands[i]
			cursor := " "
			if i == data.Selected {
				cursor = styles.CursorIcon
			}
			key := command.Key
			if key == "" {
				key = "-"
			}
			titleWidth := max(8, inner-20)
			line := cursor + " " + padRight(TruncateRunes(command.Title, titleWidth), titleWidth) + " " + padLeft(key, 8)
			if command.Description != "" && inner > 38 {
				line = fmt.Sprintf("%s  %s", line, TruncateRunes(command.Description, max(8, inner-lipgloss.Width(line)-2)))
			}
			if i == data.Selected {
				lines = append(lines, styles.DialogSelected.Render(TruncateRunes(line, inner)))
			} else {
				lines = append(lines, styles.DialogText.Render(TruncateRunes(line, inner)))
			}
		}
		if end < len(data.Commands) {
			lines = append(lines, styles.DialogMuted.Render(fmt.Sprintf("... %d more", len(data.Commands)-end)))
		}
	}
	return renderDialogBox(lines, width, height, inner)
}

func RenderCommandPanel(data CommandDialogData, width, maxHeight int) string {
	if width <= 0 || maxHeight <= 0 {
		return ""
	}
	inner := max(10, width-2)
	maxItems := max(1, min(len(data.Commands), maxHeight-4))
	start := 0
	if data.Selected >= maxItems {
		start = data.Selected - maxItems + 1
	}
	end := min(len(data.Commands), start+maxItems)

	var lines []string
	header := " Commands"
	if strings.TrimSpace(data.Query) != "" {
		header += " / " + data.Query
	}
	lines = append(lines, styles.DialogTitle.Render(padRight(header, inner)))
	if len(data.Commands) == 0 {
		lines = append(lines, styles.DialogMuted.Render(padRight("No commands available", inner)))
	} else {
		for i := start; i < end; i++ {
			command := data.Commands[i]
			cursor := " "
			if i == data.Selected {
				cursor = styles.CursorIcon
			}
			key := command.Key
			if key == "" {
				key = "-"
			}
			titleWidth := max(8, inner-16)
			line := cursor + " " + padRight(TruncateRunes(command.Title, titleWidth), titleWidth) + " " + padLeft(key, 8)
			if i == data.Selected {
				lines = append(lines, styles.DialogSelected.Render(TruncateRunes(line, inner)))
			} else {
				lines = append(lines, styles.DialogText.Render(TruncateRunes(line, inner)))
			}
		}
	}
	help := " Enter run  Esc close  Up/Down move"
	lines = append(lines, styles.DialogMuted.Render(TruncateRunes(help, inner)))
	for i, line := range lines {
		lines[i] = "│" + padRight(line, inner) + "│"
	}
	top := styles.DialogBorder.Render("╭" + strings.Repeat("─", inner) + "╮")
	bottom := styles.DialogBorder.Render("╰" + strings.Repeat("─", inner) + "╯")
	return top + "\n" + strings.Join(lines, "\n") + "\n" + bottom
}

func RenderSearchResults(data SearchResultsData, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	dialogWidth := min(max(58, width*2/3), max(20, width-4))
	inner := max(10, dialogWidth-4)
	maxItems := max(1, min(len(data.Results), height-8))
	start := 0
	if data.Selected >= maxItems {
		start = data.Selected - maxItems + 1
	}
	end := min(len(data.Results), start+maxItems)

	var lines []string
	lines = append(lines, styles.DialogTitle.Render("Search History"))
	lines = append(lines, styles.DialogMuted.Render("Enter switch  i insert  Esc close  Up/Down move"))
	query := data.Query
	if query == "" {
		query = " "
	}
	lines = append(lines, styles.DialogText.Render("query: "+TruncateRunes(query, inner-7)))
	lines = append(lines, "")
	if len(data.Results) == 0 {
		lines = append(lines, styles.DialogMuted.Render("No matching messages"))
	} else {
		for i := start; i < end; i++ {
			result := data.Results[i]
			cursor := " "
			if i == data.Selected {
				cursor = styles.CursorIcon
			}
			header := cursor + " " +
				padRight(TruncateRunes(result.Metadata.SessionID, 12), 12) + " " +
				padRight(string(result.Message.Role), 9) + " " +
				TruncateRunes(result.Metadata.Title, max(8, inner-27))
			preview := strings.Join(strings.Fields(result.Preview), " ")
			if preview != "" {
				header += "  " + TruncateRunes(preview, max(8, inner-lipgloss.Width(header)-2))
			}
			if i == data.Selected {
				lines = append(lines, styles.DialogSelected.Render(TruncateRunes(header, inner)))
			} else {
				lines = append(lines, styles.DialogText.Render(TruncateRunes(header, inner)))
			}
		}
		if end < len(data.Results) {
			lines = append(lines, styles.DialogMuted.Render(fmt.Sprintf("... %d more", len(data.Results)-end)))
		}
	}
	return renderDialogBox(lines, width, height, inner)
}

func RenderSessionDialog(data SessionDialogData, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	dialogWidth := min(max(44, width/2), max(20, width-4))
	inner := max(10, dialogWidth-4)
	maxItems := max(1, min(len(data.Sessions), height-8))

	start := 0
	if data.Selected >= maxItems {
		start = data.Selected - maxItems + 1
	}
	end := min(len(data.Sessions), start+maxItems)

	var lines []string
	lines = append(lines, styles.DialogTitle.Render("Sessions"))
	lines = append(lines, styles.DialogMuted.Render("Enter switch  Esc close  Up/Down move"))
	lines = append(lines, "")
	if len(data.Sessions) == 0 {
		lines = append(lines, styles.DialogMuted.Render("No sessions yet"))
	} else {
		for i := start; i < end; i++ {
			meta := data.Sessions[i]
			cursor := " "
			if i == data.Selected {
				cursor = styles.CursorIcon
			}
			current := " "
			if meta.SessionID == data.Current {
				current = "*"
			}
			titleWidth := max(8, inner-28)
			line := cursor + current + " " +
				padRight(TruncateRunes(meta.SessionID, 12), 12) + " " +
				padRight(TruncateRunes(meta.Title, titleWidth), titleWidth) + " " +
				padLeft(fmt.Sprintf("%d", meta.MessageCount), 3) + " msg " +
				padLeft(fmt.Sprintf("%d", meta.FileCount), 2) + " files"
			if i == data.Selected {
				lines = append(lines, styles.DialogSelected.Render(TruncateRunes(line, inner)))
			} else {
				lines = append(lines, styles.DialogText.Render(TruncateRunes(line, inner)))
			}
		}
		if end < len(data.Sessions) {
			lines = append(lines, styles.DialogMuted.Render(fmt.Sprintf("... %d more", len(data.Sessions)-end)))
		}
	}

	return renderDialogBox(lines, width, height, inner)
}

func renderDialogBox(lines []string, width, height, inner int) string {
	for i, line := range lines {
		lines[i] = " " + padRight(line, inner) + " "
	}
	body := strings.Join(lines, "\n")
	box := styles.DialogBorder.Render("╭"+strings.Repeat("─", inner+2)+"╮") + "\n" +
		body + "\n" +
		styles.DialogBorder.Render("╰"+strings.Repeat("─", inner+2)+"╯")
	return centerBlock(box, width, height)
}

func centerBlock(block string, width, height int) string {
	lines := strings.Split(block, "\n")
	blockWidth := 0
	for _, line := range lines {
		blockWidth = max(blockWidth, lipgloss.Width(line))
	}
	leftPad := max(0, (width-blockWidth)/2)
	topPad := max(0, (height-len(lines))/2)
	padded := make([]string, 0, topPad+len(lines))
	for range topPad {
		padded = append(padded, "")
	}
	prefix := strings.Repeat(" ", leftPad)
	for _, line := range lines {
		padded = append(padded, prefix+line)
	}
	return strings.Join(padded, "\n")
}

func padLeft(s string, width int) string {
	return strings.Repeat(" ", max(0, width-lipgloss.Width(s))) + s
}
