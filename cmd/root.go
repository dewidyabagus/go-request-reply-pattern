package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:  "myapp",
	Long: "Implementation of Request-Reply Pattern",
}

func init() {
	rootCmd.AddCommand(
		clientCmd, serverCmd,
	)
}

func Execute() error {
	return rootCmd.Execute()
}
