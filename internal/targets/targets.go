// Package targets implements all target APIs of Iris.
package targets

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
	//	targets <subcommand>
	//	targets all
	//	targets [--with-conent] key <key>...
	//	targets upload [--probe] <file>
	//	targets delete <key>
	cmdName         = "targets"
	subcmdNames     = []string{"all", "key", "upload", "delete"}
	fKeyWithContent bool
	fUploadProbe    bool

	// Test code can change Fatal to Panic, allowing recovery
	// from a fatal error without causing the process to exit.
	fatal    = log.Fatal
	cliFatal = common.CliFatal
	verbose  = common.Verbose
)

// TargetsCmd returns the command structure for targets.
func TargetsCmd() *cobra.Command {
	targetsCmd := &cobra.Command{
		Use:       cmdName,
		ValidArgs: subcmdNames,
		Short:     "targets API commands",
		Long:      "targets API commands for getting, uploading, and deleting target-lists and probe-lists",
		Args:      targetsArgs,
		Run:       targets,
	}
	targetsCmd.SetUsageFunc(common.Usage)
	targetsCmd.SetHelpFunc(common.Help)

	// targets all (has no flags)
	allSubcmd := &cobra.Command{
		Use:   "all",
		Short: "get all target-lists",
		Long:  "get all target-lists of the current user",
		Args:  targetsAllArgs,
		Run:   targetsAll,
	}
	targetsCmd.AddCommand(allSubcmd)

	// targets key and its flags
	keySubcmd := &cobra.Command{
		Use:   "key",
		Short: "get target-list(s) specified by key(s)",
		Long:  "get target-list(s) specified by key(s)",
		Args:  targetsKeyArgs,
		Run:   targetsKey,
	}
	keySubcmd.Flags().BoolVar(&fKeyWithContent, "with-content", false, "with target-list content")
	targetsCmd.AddCommand(keySubcmd)

	// targets upload and its flags.
	uploadSubcmd := &cobra.Command{
		Use:   "upload",
		Short: "upload a target-list or a probe-list file",
		Long:  "upload a target-list or a probe-list file",
		Args:  targetsUploadArgs,
		Run:   targetsUpload,
	}
	uploadSubcmd.Flags().BoolVar(&fUploadProbe, "probe", false, "upload a probes-list file")
	targetsCmd.AddCommand(uploadSubcmd)

	// targets delete and its flags
	deleteSubcmd := &cobra.Command{
		Use:   "delete",
		Short: "delete target-list(s) specified by key(s)",
		Long:  "delete target-list(s) specified by key(s)",
		Args:  targetsDeleteArgs,
		Run:   targetsDelete,
	}
	targetsCmd.AddCommand(deleteSubcmd)

	return targetsCmd
}

func targetsArgs(cmd *cobra.Command, args []string) error {
	if _, ok := common.IsUsage(args); ok {
		return nil
	}
	if len(args) == 0 {
		cliFatal("targets requires one of these subcommands: ", strings.Join(subcmdNames, " "))
	}
	cliFatal("unknown subcommand: ", args[0])
	return nil
}

func targets(cmd *cobra.Command, args []string) {
	fatal("targets()")
}

func targetsAllArgs(cmd *cobra.Command, args []string) error {
	if _, ok := common.IsUsage(args); ok {
		return nil
	}
	if len(args) != 0 {
		cliFatal("targets all does not take any arguments")
	}
	return nil
}

func targetsAll(cmd *cobra.Command, args []string) {
	if _, err := getAll(); err != nil {
		fatal(err)
	}
}

func targetsKeyArgs(cmd *cobra.Command, args []string) error {
	if format, ok := common.IsUsage(args); ok {
		fmt.Printf(format, "<key>...", "key(s) specifying a target-list(s)")
		return nil
	}
	if len(args) < 1 {
		cliFatal("targets key requires at least one argument: <key>...")
	}
	return nil
}

func targetsKey(cmd *cobra.Command, args []string) {
	for _, arg := range args {
		if _, err := getByKey(arg); err != nil {
			fatal(err)
		}
	}
}

func targetsUploadArgs(cmd *cobra.Command, args []string) error {
	if format, ok := common.IsUsage(args); ok {
		fmt.Printf(format, "<file>", "probe-list file or taget-list file")
		return nil
	}
	if len(args) != 1 {
		if fUploadProbe {
			cliFatal("targets upload --probe requires exactly one argument: <probe-list-file>", common.ProbeListFile)
		}
		cliFatal("targets upload requires exactly one argument: <target-list-file>", common.TargetListFile)
	}
	return nil
}

func targetsUpload(cmd *cobra.Command, args []string) {
	for _, arg := range args {
		if _, err := common.CheckFile("target-list", arg); err != nil {
			fatal(err)
		}
		if err := postList(arg); err != nil {
			fatal(err)
		}
	}
}

func targetsDeleteArgs(cmd *cobra.Command, args []string) error {
	if format, ok := common.IsUsage(args); ok {
		fmt.Printf(format, "<key>...", "key(s) specifying a target-list(s)")
		return nil
	}
	if len(args) < 1 {
		cliFatal("targets delete requires at least one argument: <key>...")
	}
	return nil
}

func targetsDelete(cmd *cobra.Command, args []string) {
	for _, arg := range args {
		if err := deleteByKey(arg); err != nil {
			fatal(err)
		}
	}
}

func getAll() ([]byte, error) {
	url := fmt.Sprintf("%s/?&offset=0&limit=200", common.TargetsAPI)
	return getResults(url, true)
}

func getByKey(key string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s?with_content=%v", common.TargetsAPI, key, fKeyWithContent)
	return getResults(url, true)
}

func postList(file string) error {
	url := fmt.Sprintf("%v/", common.TargetsAPI)
	if fUploadProbe {
		url = url + "/probes/"
	}
	jsonData, err := common.Curl(auth.GetAccessToken(), false, "POST", url,
		"-H", "Content-Type: multipart/form-data",
		"-F", fmt.Sprintf("target_file=@%v;type=text/csv", file),
	)
	if err != nil {
		return err
	}
	fmt.Printf("response: %v\n", string(jsonData))
	return nil
}

func deleteByKey(key string) error {
	fmt.Println("targets delete not implemented yet")
	return nil
}

func getResults(url string, pr bool) ([]byte, error) {
	jsonData, err := common.Curl(auth.GetAccessToken(), false, "GET", url)
	if err != nil {
		fmt.Println(string(jsonData))
		return nil, err
	}
	file, err := common.WriteResults("irisctl-targets", jsonData)
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
