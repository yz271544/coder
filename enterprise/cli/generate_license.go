package cli

import (
	"fmt"
	"os"

	"github.com/coder/coder/v2/enterprise/coderd/license"
	"github.com/coder/serpent"
)

func (r *RootCmd) generateLicense() *serpent.Command {
	var (
		outputFile string
	)

	cmd := &serpent.Command{
		Use:   "generate-license",
		Short: "Generate an offline license for Coder",
		Long:  "Generate a 10-year offline license for use in air-gapped environments.",
		Handler: func(inv *serpent.Invocation) error {
			licenseString, _, _ := license.GenerateOfflineLicense()

			if outputFile == "" {
				// 输出到标准输出
				fmt.Fprintln(inv.Stdout, licenseString)
			} else {
				// 输出到文件
				err := os.WriteFile(outputFile, []byte(licenseString), 0644)
				if err != nil {
					return fmt.Errorf("failed to write license to file: %w", err)
				}
				fmt.Fprintf(inv.Stdout, "License written to %s\n", outputFile)
			}

			return nil
		},
	}

	cmd.Options = serpent.OptionSet{
		{
			Flag:          "output",
			FlagShorthand: "o",
			Env:           "CODER_LICENSE_OUTPUT",
			Description:   "Output file path for the license. If not specified, prints to stdout.",
			Value:         serpent.StringOf(&outputFile),
		},
	}

	return cmd
}
