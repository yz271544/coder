//go:build slim

package cli

import (
	agplcli "github.com/coder/coder/v2/cli"
	"github.com/coder/serpent"
)

func (r *RootCmd) generateLicense() *serpent.Command {
	cmd := &serpent.Command{
		Use:     "generate-license",
		Short:   "Generate an offline license for Coder",
		RawArgs: true,
		Hidden:  false,
		Handler: func(inv *serpent.Invocation) error {
			agplcli.SlimUnsupported(inv.Stderr, "generate license")
			return nil
		},
	}

	return cmd
}
