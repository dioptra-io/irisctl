// Package status implements all status APIs of Iris.
package status

import (
	"fmt"
	"log"
	"os"

	"github.com/dioptra-io/irisctl/internal/auth"
	"github.com/dioptra-io/irisctl/internal/common"
	"github.com/spf13/cobra"
)

var (
	// Command, its flags, subcommands, and their flags.
	//      status (has no flags)
	cmdName     = "status"
	subcmdNames = []string{}

	// Test code changes Fatal to Panic so a fatal error won't exit
	// the process and can be recovered.
	fatal    = log.Fatal
	cliFatal = common.CliFatal
	verbose  = common.Verbose
)

func StatusCmd() *cobra.Command {
	// status (has no subcommands or flags)
	statusCmd := &cobra.Command{
		Use:       cmdName,
		ValidArgs: subcmdNames,
		Short:     "status API commands",
		Long:      "status API commands for getting status of Iris",
		Args:      statusArgs,
		Run:       status,
	}
	statusCmd.SetUsageFunc(common.Usage)
	statusCmd.SetHelpFunc(common.Help)

	return statusCmd
}

func statusArgs(cmd *cobra.Command, args []string) error {
	if _, ok := common.IsUsage(args); ok {
		return nil
	}
	if len(args) != 0 {
		cliFatal("status does not take any arguments")
	}
	return nil
}

func status(cmd *cobra.Command, args []string) {
	if _, err := getResults(common.APIEndpoint(common.StatusAPISuffix), true); err != nil {
		fatal(err)
	}
}

func getResults(url string, pr bool) ([]byte, error) {
	jsonData, err := common.Curl(auth.GetAccessToken(), false, "GET", url+"/")
	if err != nil {
		fmt.Println(string(jsonData))
		return nil, err
	}
	file, err := common.WriteResults("irisctl-status", jsonData)
	if !common.RootFlagBool("no-delete") {
		defer func(f string) { verbose("removing %s\n", f); os.Remove(f) }(file)
	}
	if err != nil {
		return nil, err
	}
	filter := []string{"."}
	jqOutput, err := common.JqFile(file, filter)
	if pr {
		fmt.Println(string(jqOutput))
	}
	return jsonData, err
}
