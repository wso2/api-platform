package apiproject

import "github.com/spf13/cobra"

const (
	ApiProjectCmdLiteral = "apiproject"
	ApiProjectCmdExample = `# Add a new API project
ap apiproject init --display-name foo-api --type rest --version 1.0 --context /foo`
)

var ApiProjectCmd = &cobra.Command{
	Use:     ApiProjectCmdLiteral,
	Short:   "Execute API project operations",
	Long:    "This command allows you to manage API projects.",
	Example: ApiProjectCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	ApiProjectCmd.AddCommand(initCmd)
}