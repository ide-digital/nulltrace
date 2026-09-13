package tui

import tea "github.com/charmbracelet/bubbletea"

type vaultForm struct {
	hint string
}

func newVaultForm() vaultForm {
	return vaultForm{
		hint: "Passphrase is entered on the CLI, not here. Tab away when you're done looking.",
	}
}

func (v vaultForm) View() string {
	return styleTitle.Render("  vault") + "\n\n  " + styleMuted.Render(v.hint)
}

func (v vaultForm) Update(msg tea.Msg) (vaultForm, tea.Cmd) {
	return v, nil
}
