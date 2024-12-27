// Package maint implements all maintenance APIs of Iris.
package maint

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/dioptra-io/irisctl/internal/auth"
	"github.com/dioptra-io/irisctl/internal/common"
	"github.com/spf13/cobra"
)

var (
	// Command, its flags, subcommands, and their flags.
	//      maint dq <queue-name>...
	//      maint dq --post <queue-name> [<actor-string>]  (XXX actor-string: watch_measurement_agent)
	//      maint dq --delete <queue-name> <redis-message-id>
	//      maint meas delete <meas-uuid>
	cmdName     = "maint"
	subcmdNames = []string{"dq", "meas"}
	fDqPost     bool
	fDqDelete   bool

	// Test code changes Fatal to Panic so a fatal error won't exit
	// the process and can be recovered.
	fatal    = log.Fatal
	cliFatal = common.CliFatal
	verbose  = common.Verbose
)

func MaintCmd() *cobra.Command {
	// maint (has no flags)
	maintCmd := &cobra.Command{
		Use:       cmdName,
		ValidArgs: subcmdNames,
		Short:     "maintenance API commands",
		Long:      "maintenance API commands for getting,  posting, and deleting dramatiq messages, and deleting measurements",
		Args:      maintArgs,
		Run:       maint,
	}
	maintCmd.SetUsageFunc(common.Usage)
	maintCmd.SetHelpFunc(common.Help)

	// maint dq
	dqSubcmd := &cobra.Command{
		Use:   "dq",
		Short: "get, post, or delete dramatiq message(s)",
		Long:  "get, post, or delete dramatiq message(s)",
		Args:  maintDqArgs,
		Run:   maintDq,
	}
	dqSubcmd.Flags().BoolVar(&fDqPost, "post", false, "post dramatiq queue")
	dqSubcmd.Flags().BoolVar(&fDqDelete, "delete", false, "delete dramatiq queue")
	maintCmd.AddCommand(dqSubcmd)

	// maint meas delete
	measSubcmd := &cobra.Command{
		Use:   "meas",
		Short: "delete measurement(s)",
		Long:  "delete measurement(s) specified by measurement UUID(s)",
		Args:  maintMeasArgs,
		Run:   maintMeas,
	}
	maintCmd.AddCommand(measSubcmd)

	return maintCmd
}

func maintArgs(cmd *cobra.Command, args []string) error {
	if _, ok := common.IsUsage(args); ok {
		return nil
	}
	if len(args) == 0 {
		cliFatal("maint requires one of these subcommands: ", strings.Join(subcmdNames, " "))
	}
	cliFatal("unknown subcommand: ", args[0])
	return nil
}

func maint(cmd *cobra.Command, args []string) {
	fatal("maint()")
}

func maintDqArgs(cmd *cobra.Command, args []string) error {
	if format, ok := common.IsUsage(args); ok {
		fmt.Printf(format, "<queue-name>...", "name of dramatiq queue(s)")
		return nil
	}
	if len(args) == 0 {
		cliFatal("maint dq requires at least one argument: <queue-name>...")
	}
	if fDqPost && fDqDelete {
		cliFatal("specify either --post or --delete")
	}
	if fDqPost && (len(args) < 1 || len(args) > 2) {
		cliFatal("maint dq --post requires at least one argument: <queue-name> [<actor-string>]")
	}
	if fDqDelete && len(args) != 2 {
		cliFatal("maint dq --delete requires exactly two arguments: <queue-name> <redis-message-id>")
	}
	return nil
}

func maintDq(cmd *cobra.Command, args []string) {
	if !fDqPost && !fDqDelete {
		for _, arg := range args {
			verbose("%v:\n", arg)
			if err := getMaintenanceDq(arg); err != nil {
				fatal(err)
			}
		}
	}
	if fDqPost {
		actor := ""
		if len(args) > 1 {
			actor = args[1]
		}
		if err := postMaintenanceDq(args[0], actor); err != nil {
			fatal(err)
		}
	}
	if fDqDelete {
		if err := deleteMaintenanceDq(args[0], args[1]); err != nil {
			fatal(err)
		}
	}
}

func maintMeasArgs(cmd *cobra.Command, args []string) error {
	if format, ok := common.IsUsage(args); ok {
		fmt.Printf(format, "<meas-uuid>...", "measurement UUID")
		return nil
	}
	if len(args) < 2 || args[0] != "delete" {
		cliFatal("maint meas requires an explicit \"delete\" and at least one argument: <meas-uuid>...")
	}
	if err := common.ValidateFormat(args[1:], common.MeasurementUUID); err != nil {
		cliFatal(err)
	}
	return nil
}

func maintMeas(cmd *cobra.Command, args []string) {
	for _, arg := range args[1:] {
		if err := deleteMaintenanceMeas(arg); err != nil {
			fatal(err)
		}
	}
}

func getMaintenanceDq(queue string) error {
	fmt.Printf("maint dq not implemented yet (queue=%v)\n", queue)
	return nil
}

func postMaintenanceDq(queue, actor string) error {
	fmt.Printf("maint dq --post not implemented yet (queue=%v actor=%s)\n", queue, actor)
	return nil
}

func deleteMaintenanceDq(queue, redisMsgId string) error {
	fmt.Printf("maint dq --delete not implemented yet (queue=%v redisMsgId=%v)\n", queue, redisMsgId)
	return nil
}

func deleteMaintenanceMeas(measUUID string) error {
	f, err := os.CreateTemp("/tmp", "irisctl-maint-meas-delete-")
	if err != nil {
		return err
	}
	defer f.Close()
	fmt.Fprintf(os.Stderr, "saving in %s\n", f.Name())

	url := fmt.Sprintf("%s/measurements/%s", common.APIEndpoint((common.MaintenanceAPISuffix)), measUUID)
	jsonData, err := common.Curl(auth.GetAccessToken(), false, "DELETE", url)
	if err != nil {
		return err
	}
	if _, err := f.Write(jsonData); err != nil {
		return err
	}
	return nil
}
