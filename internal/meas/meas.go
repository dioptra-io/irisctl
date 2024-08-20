// Package meas implements all measurement APIs of Iris.
package meas

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/dioptra-io/irisctl/internal/auth"
	"github.com/dioptra-io/irisctl/internal/common"
	"github.com/spf13/cobra"
)

var (
	// Command, its flags, subcommands, and their flags.
	//	meas [--state <state>] [--tag <tag>] [--all-users] [--public]
	//	meas --uuid <measurement-uuid>...
	//	meas --target-list <measurement-uuid> <agent-uuid>
	//	meas request <meas-file>...
	//	meas delete <measurement-uuid>...
	//	meas edit <measurement-uuid> <patch-file>
	cmdName         = "meas"
	subcmdNames     = []string{"request", "delete", "edit"}
	fMeasState      string
	fMeasTag        string
	fMeasAllUsers   bool
	fMeasPublic     bool
	fMeasUUID       bool
	fMeasTargetList bool

	// Test code changes Fatal to Panic so a fatal error won't exit
	// the process and can be recovered.
	fatal    = log.Fatal
	cliFatal = common.CliFatal
	verbose  = common.Verbose
)

// MeasCmd returns the command structure for measurements.
func MeasCmd() *cobra.Command {
	measCmd := &cobra.Command{
		Use:       cmdName,
		ValidArgs: subcmdNames,
		Short:     "measurements API commands",
		Long:      "measurements API commands for getting, requesting, and canceling measurements",
		Args:      measArgs,
		Run:       meas,
	}
	measCmd.Flags().StringVarP(&fMeasState, "state", "", "", "get measurements with the specified state")
	measCmd.Flags().StringVarP(&fMeasTag, "tag", "", "", "get measurements with the specified tag")
	measCmd.Flags().BoolVarP(&fMeasAllUsers, "all-users", "", false, "get all measurements of all users (admin only)")
	measCmd.Flags().BoolVarP(&fMeasPublic, "public", "", false, "get measurements tagged as visibility:public")
	measCmd.Flags().BoolVarP(&fMeasUUID, "uuid", "", false, "get measurements with the specified UUIDs")
	measCmd.Flags().BoolVarP(&fMeasTargetList, "target-list", "", false, "get the target-list of the specified measurement and agent")
	measCmd.SetUsageFunc(common.Usage)
	measCmd.SetHelpFunc(common.Help)

	// meas request (has no flags)
	requestSubcmd := &cobra.Command{
		Use:   "request",
		Short: "request measurement(s)",
		Long:  "request measurement(s) with details in the specified file(s)",
		Args:  measRequestArgs,
		Run:   measRequest,
	}
	measCmd.AddCommand(requestSubcmd)

	// meas delete (has no flags)
	deleteSubcmd := &cobra.Command{
		Use:   "delete",
		Short: "delete measurement(s)",
		Long:  "delete measurement(s) specified by measurement UUID(s)",
		Args:  measDeleteArgs,
		Run:   measDelete,
	}
	measCmd.AddCommand(deleteSubcmd)

	// meas edit (has no flags)
	editSubcmd := &cobra.Command{
		Use:   "edit",
		Short: "edit a measurement",
		Long:  "edit the specified measurement with details in the specified file",
		Args:  measEditArgs,
		Run:   measEdit,
	}
	measCmd.AddCommand(editSubcmd)

	return measCmd
}

func GetMeasMdFile(allUsers bool) (string, error) {
	fMeasAllUsers = allUsers
	return getMeasMdFile()
}

func measArgs(cmd *cobra.Command, args []string) error {
	if _, ok := common.IsUsage(args); ok {
		return nil
	}
	if !fMeasUUID && !fMeasTargetList && len(args) != 0 {
		cliFatal("meas does not take any arguments")
	}
	if fMeasTargetList && fMeasUUID {
		cliFatal("specify either --target-list or --uuid")
	}
	if fMeasUUID && len(args) < 1 {
		cliFatal("meas --uuid requires at least one argument: <measurement-uuid>...")
	}
	if fMeasTargetList && len(args) != 2 {
		cliFatal("meas --target-list requires two arguments: <measurement-uuid> <agent-uuid>")
	}
	return nil
}

func meas(cmd *cobra.Command, args []string) {
	if fMeasTargetList {
		if err := getTargetList(args[0], args[1]); err != nil {
			fatal(err)
		}
		return
	}
	if fMeasUUID {
		for _, arg := range args {
			if err := getMeasurementByUUID(arg); err != nil {
				fatal(err)
			}
			fmt.Println()
		}
		return
	}
	if _, err := getMeasMdFile(); err != nil {
		fatal(err)
	}
}

func measRequestArgs(cmd *cobra.Command, args []string) error {
	if format, ok := common.IsUsage(args); ok {
		fmt.Printf(format, "<meas-file>", "measurement definition file")
		return nil
	}
	if len(args) < 1 {
		cliFatal("meas request requires at least one argument: <meas-file>...", common.MeasurementFile)
	}
	for _, arg := range args {
		if _, err := common.CheckFile("measurement file", arg); err != nil {
			fatal(err)
		}
	}
	return nil
}

func measRequest(cmd *cobra.Command, args []string) {
	if err := postMeasurementRequst(args[0]); err != nil {
		fatal(err)
	}
}

func measDeleteArgs(cmd *cobra.Command, args []string) error {
	if format, ok := common.IsUsage(args); ok {
		fmt.Printf(format, "<measurement-uuid>...", "measurement UUID")
		return nil
	}
	if len(args) < 1 {
		cliFatal("meas delete requires at least one argument: <measurement-uuid>...")
	}
	if err := common.ValidateFormat(args, common.MeasurementUUID); err != nil {
		cliFatal(err)
	}
	return nil
}

func measDelete(cmd *cobra.Command, args []string) {
	for _, measUUID := range args {
		if err := deleteMeasurement(measUUID); err != nil {
			fatal(err)
		}
	}
}

func measEditArgs(cmd *cobra.Command, args []string) error {
	if format, ok := common.IsUsage(args); ok {
		fmt.Printf(format, "<measurement-uuid> <patch-file>", "measurement UUID and details in the patch file")
		return nil
	}
	if len(args) != 2 {
		cliFatal("meas edit requires two arguments: <measurement-uuid> <patch-file>")
	}
	if err := common.ValidateFormat(args, common.MeasurementUUID); err != nil {
		cliFatal(err)
	}
	return nil
}

func measEdit(cmd *cobra.Command, args []string) {
	if err := patchMeasurement(); err != nil {
		fatal(err)
	}
}

func getTargetList(measUUID, agentUUID string) error {
	url := fmt.Sprintf("%s/%s/%s/target", common.MeasurementsAPI, measUUID, agentUUID)
	jsonData, err := common.Curl(auth.GetAccessToken(), false, "GET", url)
	if err != nil {
		return err
	}
	return common.SaveOrPrint(jsonData, "irisctl-meas-target-")
}

func getMeasurementByUUID(uuid string) error {
	url := fmt.Sprintf("%s/%s", common.MeasurementsAPI, uuid)
	jsonData, err := common.Curl(auth.GetAccessToken(), false, "GET", url)
	if err != nil {
		return err
	}
	return common.SaveOrPrint(jsonData, "irisctl-meas-uuid-")
}

func getMeasMdFile() (string, error) {
	var prefix string
	if fMeasAllUsers {
		verbose("getting metadata of all measurements\n")
		prefix = "irisctl-meas-all-"
	} else {
		verbose("getting metadata of my measurements\n")
		prefix = "irisctl-meas-me-"
	}
	f, err := os.CreateTemp("/tmp", prefix)
	if err != nil {
		return "", err
	}
	defer f.Close()
	fmt.Fprintf(os.Stderr, "saving in %s\n", f.Name())

	limit := 200
	defer fmt.Println()
	for offset := 0; offset < 10000; offset += limit {
		verbose("getting from offset %d to %d\r", offset, offset+limit)
		url := common.MeasurementsAPI
		if fMeasPublic {
			url = fmt.Sprintf("%s/public?", url)
		} else {
			url = fmt.Sprintf("%s/?only_mine=%v&", url, !fMeasAllUsers)
		}
		if fMeasState != "" {
			url = fmt.Sprintf("%sstate=%v&", url, fMeasState)
		}
		if fMeasTag != "" {
			url = fmt.Sprintf("%stag=%v&", url, fMeasTag)
		}
		url = fmt.Sprintf("%soffset=%d&limit=%d", url, offset, limit)
		jsonData, err := common.Curl(auth.GetAccessToken(), false, "GET", url)
		if err != nil {
			return f.Name(), err
		}
		if _, err := f.Write(jsonData); err != nil {
			return f.Name(), err
		}
		var batch common.MeasurementBatch
		if err := json.Unmarshal(jsonData, &batch); err != nil {
			return f.Name(), err
		}
		if batch.Next == nil || *batch.Next == "" {
			break
		}
	}
	return f.Name(), nil
}

func postMeasurementRequst(measFile string) error {
	fmt.Println("postMeasurementRequest() request not implemented yet")
	return nil
}

func deleteMeasurement(measUUID string) error {
	url := fmt.Sprintf("%s/%s", common.MeasurementsAPI, measUUID)
	jsonData, err := common.Curl(auth.GetAccessToken(), false, "DELETE", url)
	if err != nil {
		return err
	}
	return common.SaveOrPrint(jsonData, "irisctl-meas-delete-")
}

func patchMeasurement() error {
	fmt.Println("patchMeasurement() not implemented yet")
	return nil
}
