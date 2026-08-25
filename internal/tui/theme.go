package tui

import (
	"charm.land/bubbles/v2/help"
	huh "charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// everforestTheme returns a huh theme matching the Everforest Dark Soft color
// scheme and the visual style of fzf-tmux popups.
func everforestTheme() huh.ThemeFunc {
	return func(isDark bool) *huh.Styles {
		var (
			dimmed = lipgloss.Color("8")
			red    = lipgloss.Color("1")
			green  = lipgloss.Color("2")
			blue   = lipgloss.Color("4")
			yellow = lipgloss.Color("3")
			border = lipgloss.Color("8")
		)

		var t huh.Styles

		t.Form.Base = lipgloss.NewStyle()
		t.Group.Base = lipgloss.NewStyle()
		t.FieldSeparator = lipgloss.NewStyle().SetString("\n\n")

		button := lipgloss.NewStyle().
			Padding(0, 2).
			MarginRight(1)

		// Focused styles — light left border to feel minimal like fzf.
		t.Focused.Base = lipgloss.NewStyle().
			PaddingLeft(1).
			BorderStyle(lipgloss.NormalBorder()).
			BorderLeft(true).
			BorderForeground(border)
		t.Focused.Card = t.Focused.Base
		t.Focused.Title = lipgloss.NewStyle().Foreground(blue).Bold(true)
		t.Focused.NoteTitle = lipgloss.NewStyle().Foreground(blue).Bold(true).MarginBottom(1)
		t.Focused.Description = lipgloss.NewStyle().Foreground(dimmed)
		t.Focused.ErrorIndicator = lipgloss.NewStyle().Foreground(red).SetString(" *")
		t.Focused.ErrorMessage = lipgloss.NewStyle().Foreground(red)
		t.Focused.Directory = lipgloss.NewStyle().Foreground(blue)
		t.Focused.File = lipgloss.NewStyle()

		// Select styles — red selector like fzf prompt color.
		t.Focused.SelectSelector = lipgloss.NewStyle().Foreground(red).SetString("> ")
		t.Focused.NextIndicator = lipgloss.NewStyle().MarginLeft(1).Foreground(red).SetString("->")
		t.Focused.PrevIndicator = lipgloss.NewStyle().MarginRight(1).Foreground(red).SetString("<-")
		t.Focused.Option = lipgloss.NewStyle()

		// Multi-select styles.
		t.Focused.MultiSelectSelector = lipgloss.NewStyle().Foreground(red).SetString("> ")
		t.Focused.SelectedOption = lipgloss.NewStyle().Foreground(green)
		t.Focused.SelectedPrefix = lipgloss.NewStyle().Foreground(green).SetString("[x] ")
		t.Focused.UnselectedOption = lipgloss.NewStyle()
		t.Focused.UnselectedPrefix = lipgloss.NewStyle().Foreground(dimmed).SetString("[ ] ")

		// Button styles.
		t.Focused.FocusedButton = button.Foreground(lipgloss.Color("0")).Background(green).Bold(true)
		t.Focused.Next = t.Focused.FocusedButton
		t.Focused.BlurredButton = button.Background(lipgloss.Color("0"))

		// Text input styles — red prompt, green cursor.
		t.Focused.TextInput.Cursor = lipgloss.NewStyle().Foreground(green)
		t.Focused.TextInput.CursorText = lipgloss.NewStyle()
		t.Focused.TextInput.Placeholder = lipgloss.NewStyle().Foreground(dimmed)
		t.Focused.TextInput.Prompt = lipgloss.NewStyle().Foreground(yellow)
		t.Focused.TextInput.Text = lipgloss.NewStyle()

		t.Help = help.New().Styles

		// Blurred styles — same but hidden border.
		t.Blurred = t.Focused
		t.Blurred.Base = t.Blurred.Base.BorderStyle(lipgloss.HiddenBorder())
		t.Blurred.Card = t.Blurred.Base
		t.Blurred.NextIndicator = lipgloss.NewStyle()
		t.Blurred.PrevIndicator = lipgloss.NewStyle()

		t.Group.Title = t.Focused.Title
		t.Group.Description = t.Focused.Description

		return &t
	}
}
