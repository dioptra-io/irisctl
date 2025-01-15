package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/dioptra-io/irisctl/internal/agents"
	"github.com/dioptra-io/irisctl/internal/auth"
	"github.com/dioptra-io/irisctl/internal/common"
	"github.com/dioptra-io/irisctl/internal/maint"
	"github.com/dioptra-io/irisctl/internal/meas"
	"github.com/dioptra-io/irisctl/internal/status"
	"github.com/dioptra-io/irisctl/internal/targets"
	"github.com/dioptra-io/irisctl/internal/users"

	"github.com/dioptra-io/irisctl/internal/analyze"
	"github.com/dioptra-io/irisctl/internal/check"
	"github.com/dioptra-io/irisctl/internal/clickhouse"
	"github.com/dioptra-io/irisctl/internal/list"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Command, its flags, subcommands, and their flags.
	//	irisctl [--brief] [--curl] [--no-delete] [--no-auto-login] [--stdout] [--verbose] <command>
	cmdName          = "irisctl"
	apiSubcmdNames   = []string{"auth", "users", "agents", "targets", "meas", "status", "maint"}
	extSubcmdNames   = []string{"api", "ext", "check", "analyze", "clickhouse", "list"}
	subcmdNames      = append(apiSubcmdNames, extSubcmdNames...)
	fRootBrief       bool
	fRootCurl        bool
	fRootNoDelete    bool
	fRootNoAutoLogin bool
	fRootStdout      bool
	fRootVerbose     bool
	fRootJqFilter    string
	fIrisAPIUrl      string
	fMeasurementUUID string

	allCmds = []*cobra.Command{}

	// Test code changes Fatal to Panic so a fatal error won't exit
	// the process and can be recovered.
	fatal    = log.Fatal
	cliFatal = common.CliFatal
)

func main() {
	irisctlCmd := &cobra.Command{
		Use:              cmdName,
		ValidArgs:        subcmdNames,
		Short:            "Iris API and extension (non-API) commands",
		Long:             "Iris API and extension (non-API) commands for checking and analyzing Iris",
		Args:             irisctlArgs,
		Run:              irisctl,
		TraverseChildren: true,
	}
	irisctlCmd.PersistentFlags().BoolVarP(&fRootBrief, "brief", "b", false, "enable brief mode (less output)")
	irisctlCmd.PersistentFlags().BoolVarP(&fRootCurl, "curl", "c", false, "show curl commands that are executed but not their output")
	irisctlCmd.PersistentFlags().BoolVarP(&fRootNoDelete, "no-delete", "d", false, "do not delete temporary files")
	irisctlCmd.PersistentFlags().BoolVarP(&fRootNoAutoLogin, "no-auto-login", "l", false, "do not auto login")
	irisctlCmd.PersistentFlags().BoolVarP(&fRootStdout, "stdout", "o", false, "print results to stdout instead of saving to a file")
	irisctlCmd.PersistentFlags().BoolVarP(&fRootVerbose, "verbose", "v", false, "enable verbose mode (more output)")
	irisctlCmd.PersistentFlags().StringVarP(&fRootJqFilter, "jq-filter", "j", ".", "jq filter")
	irisctlCmd.PersistentFlags().StringVarP(&fIrisAPIUrl, "iris-api-url", "u", "https://api.iris.dioptra.io", "specify the iris api url")
	// TODO: Instead of hard-coding a default value, we should find a measurement UUID of the user.
	irisctlCmd.PersistentFlags().StringVarP(&fMeasurementUUID, "meas-uuid", "m", "a75482d1-8c5c-4d56-845e-fc3861047992", "specify the measurement uuid for the gusethosue credentials")
	irisctlCmd.SetUsageFunc(common.Usage)
	irisctlCmd.SetHelpFunc(common.Help)

	apiCmd := &cobra.Command{
		Use:   "api",
		Short: "print iris api commands",
		Long:  "print iris api commands",
		Args:  irisctlApiArgs,
		Run:   irisctlApi,
	}
	extCmd := &cobra.Command{
		Use:   "ext",
		Short: "print extension (non-api) commands",
		Long:  "print extension (non-api) commands",
		Args:  irisctlExtArgs,
		Run:   irisctlExt,
	}

	// Bind irisctl flags so they will be globally available to
	// all commands and their subcommands.
	_ = viper.BindPFlag("brief", irisctlCmd.PersistentFlags().Lookup("brief"))
	_ = viper.BindPFlag("curl", irisctlCmd.PersistentFlags().Lookup("curl"))
	_ = viper.BindPFlag("no-delete", irisctlCmd.PersistentFlags().Lookup("no-delete"))
	_ = viper.BindPFlag("no-auto-login", irisctlCmd.PersistentFlags().Lookup("no-auto-login"))
	_ = viper.BindPFlag("stdout", irisctlCmd.PersistentFlags().Lookup("stdout"))
	_ = viper.BindPFlag("verbose", irisctlCmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("jq-filter", irisctlCmd.PersistentFlags().Lookup("jq-filter"))
	_ = viper.BindPFlag("iris-api-url", irisctlCmd.PersistentFlags().Lookup("iris-api-url"))
	_ = viper.BindPFlag("meas-uuid", irisctlCmd.PersistentFlags().Lookup("meas-uuid"))
	// Iris API commands.
	allCmds = append(allCmds, auth.AuthCmd())
	allCmds = append(allCmds, users.UsersCmd())
	allCmds = append(allCmds, agents.AgentsCmd())
	allCmds = append(allCmds, targets.TargetsCmd())
	allCmds = append(allCmds, meas.MeasCmd())
	allCmds = append(allCmds, status.StatusCmd())
	allCmds = append(allCmds, maint.MaintCmd())
	// Extension (non-API) commands.
	allCmds = append(allCmds, apiCmd)
	allCmds = append(allCmds, extCmd)
	allCmds = append(allCmds, check.CheckCmd())
	allCmds = append(allCmds, analyze.AnalyzeCmd())
	allCmds = append(allCmds, clickhouse.ClickHouseCmd())
	allCmds = append(allCmds, list.ListCmd())
	// Add all API and extension (non-API) commands.
	for _, cmd := range allCmds {
		irisctlCmd.AddCommand(cmd)
	}
	// Run the tool.
	if err := irisctlCmd.Execute(); err != nil {
		fatal(err)
	}
}

func irisctlArgs(cmd *cobra.Command, args []string) error {
	if format, ok := common.IsUsage(args); ok {
		s := fmt.Sprintf("one of these: %s", strings.Join(subcmdNames, " "))
		fmt.Printf(format, "<command>", s)
		return nil
	}
	if len(args) < 1 {
		cliFatal("irisctl requires one of these commands: ", strings.Join(subcmdNames, " "))
	}
	if !common.Contains(extSubcmdNames, args[0]) {
		cliFatal("unknown command: ", args[0])
	}
	return nil
}

func irisctl(cmd *cobra.Command, args []string) {
	fatal("irisctl()")
}

func irisctlApiArgs(cmd *cobra.Command, args []string) error {
	if _, ok := common.IsUsage(args); ok {
		return nil
	}
	if len(args) != 0 {
		cliFatal("irisctl api does not take any arguments")
	}
	return nil
}

func irisctlApi(cmd *cobra.Command, args []string) {
	fmt.Printf("iris api commands: %v\n", strings.Join(apiSubcmdNames, " "))
}

func irisctlExtArgs(cmd *cobra.Command, args []string) error {
	if _, ok := common.IsUsage(args); ok {
		return nil
	}
	if len(args) != 0 {
		cliFatal("irisctl ext does not take any arguments")
	}
	return nil
}

func irisctlExt(cmd *cobra.Command, args []string) {
	fmt.Printf("extension (non-api) commands: %v\n", strings.Join(extSubcmdNames, " "))
}
