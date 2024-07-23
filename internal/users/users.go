// Package users implements all user APIs of Iris.
package users

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/dioptra-io/irisctl/internal/auth"
	"github.com/dioptra-io/irisctl/internal/common"
	"github.com/spf13/cobra"
)

var (
	// Command, its flags, subcommands, and their flags.
	//	users <subcommand>
	//	users me
	//	users all [--verified]
	//	users delete [--dry-run] <user-id>...
	//	users patch <user-id> <user-details>
	//	users services <measurement-uuid>
	cmdName       = "users"
	subcmdNames   = []string{"me", "all", "delete", "patch", "services"}
	fAllVerified  bool
	fDeleteDryRun bool

	meServices common.MeServices

	// Test code changes Fatal to Panic so a fatal error won't exit
	// the process and can be recovered.
	fatal    = log.Fatal
	cliFatal = common.CliFatal
	verbose  = common.Verbose
)

// UsersCmd returns the command structure for users.
func UsersCmd() *cobra.Command {
	usersCmd := &cobra.Command{
		Use:       cmdName,
		ValidArgs: subcmdNames,
		Short:     "users API commands",
		Long:      "users API commands for getting, editing, and deleting users",
		Args:      usersArgs,
		Run:       users,
	}
	usersCmd.SetUsageFunc(common.Usage)
	usersCmd.SetHelpFunc(common.Help)

	// users me (has no flags)
	meSubcmd := &cobra.Command{
		Use:   "me",
		Short: "get current user",
		Long:  "get details of the current user",
		Args:  usersMeArgs,
		Run:   usersMe,
	}
	usersCmd.AddCommand(meSubcmd)

	// users all and its flags
	allSubcmd := &cobra.Command{
		Use:   "all",
		Short: "get all users",
		Long:  "get details of all users",
		Args:  usersAllArgs,
		Run:   usersAll,
	}
	allSubcmd.Flags().BoolVar(&fAllVerified, "verified", false, "verifired users")
	usersCmd.AddCommand(allSubcmd)

	// users delete (has no flags)
	deleteSubcmd := &cobra.Command{
		Use:   "delete",
		Short: "delete user(s)",
		Long:  "delete the user(s) specified by id(s)",
		Args:  usersDeleteArgs,
		Run:   usersDelete,
	}
	deleteSubcmd.Flags().BoolVar(&fDeleteDryRun, "dry-run", false, "enable dry-run mode (i.e., do not execute command)")
	usersCmd.AddCommand(deleteSubcmd)

	// users patch (has no flags)
	patchSubcmd := &cobra.Command{
		Use:   "patch",
		Short: "patch user",
		Long:  "patch the user specified by its id with the contents of the specified file",
		Args:  usersPatchArgs,
		Run:   usersPatch,
	}
	usersCmd.AddCommand(patchSubcmd)

	// users me/services (has no flags)
	servicesSubcmd := &cobra.Command{
		Use:   "services",
		Short: "get services credentials",
		Long:  "get external services credentials for the current user for the specified measurement",
		Args:  usersServicesArgs,
		Run:   usersMeServices,
	}
	usersCmd.AddCommand(servicesSubcmd)

	return usersCmd
}

// GetUserPass returns username and password obtained from
// users/me/services of Iris API.
//
// XXX For now, this function uses a hard-coded measurement UUID
//
//	but going forward it should either accept a measurement UUID
//	or find a measurement UUID of the user running this instance
//	of irisctl.
func GetUserPass() (string, error) {
	if meServices.ClickHouse.Username == "" {
		uuid := "a75482d1-8c5c-4d56-845e-fc3861047992" // zeph-gcp-daily.json
		url := fmt.Sprintf("%s/me/services?measurement_uuid=%v", common.UsersAPI, uuid)
		jsonData, err := common.Curl(auth.GetAccessToken(), false, "GET", url)
		if err != nil {
			return "", err
		}
		if err := json.Unmarshal(jsonData, &meServices); err != nil {
			return "", err
		}
	}
	return meServices.ClickHouse.Username + ":" + meServices.ClickHouse.Password, nil
}

func GetUserUUIDs() ([]byte, error) {
	return getUsersAll(false)
}

func usersArgs(cmd *cobra.Command, args []string) error {
	if _, ok := common.IsUsage(args); ok {
		return nil
	}
	if len(args) == 0 {
		cliFatal("users requires one of these subcommands: ", strings.Join(subcmdNames, " "))
	}
	cliFatal("unknown subcommand: ", args[0])
	return nil
}

func users(cmd *cobra.Command, args []string) {
	fatal("users()")
}

func usersMeArgs(cmd *cobra.Command, args []string) error {
	if _, ok := common.IsUsage(args); ok {
		return nil
	}
	if len(args) != 0 {
		cliFatal("users me does not take any arguments")
	}
	return nil
}

func usersMe(cmd *cobra.Command, args []string) {
	if _, err := getUsersMe(true); err != nil {
		fatal(err)
	}
}

func usersAllArgs(cmd *cobra.Command, args []string) error {
	if _, ok := common.IsUsage(args); ok {
		return nil
	}
	if len(args) != 0 {
		cliFatal("users all does not take any arguments")
	}
	return nil
}

func usersAll(cmd *cobra.Command, args []string) {
	if _, err := getUsersAll(true); err != nil {
		fatal(err)
	}
}

func usersDeleteArgs(cmd *cobra.Command, args []string) error {
	if format, ok := common.IsUsage(args); ok {
		fmt.Printf(format, "<user-id>...", "one or more user IDs")
		return nil
	}
	if len(args) < 1 {
		cliFatal("users delete requires at least one argument: <user-id>...")
	}
	if err := common.ValidateFormat(args, common.UserID); err != nil {
		cliFatal(err)
	}
	return nil
}

func usersDelete(cmd *cobra.Command, args []string) {
	for _, arg := range args {
		if err := deleteUsersById(arg); err != nil {
			fatal(err)
		}
	}
}

func usersPatchArgs(cmd *cobra.Command, args []string) error {
	if format, ok := common.IsUsage(args); ok {
		fmt.Printf(format, "<user-id> <user-details>", "user ID and user details in JSON format")
		return nil
	}
	if len(args) != 2 {
		cliFatal("users patch requires two arguments: <user-id> <user-details>", common.UserFile)
	}
	if err := common.ValidateFormat([]string{args[0]}, common.UserID); err != nil {
		cliFatal(err)
	}
	return nil
}

func usersPatch(cmd *cobra.Command, args []string) {
	if err := patchUsersId(args[0], args[1]); err != nil {
		fatal(err)
	}
}

func usersServicesArgs(cmd *cobra.Command, args []string) error {
	if format, ok := common.IsUsage(args); ok {
		fmt.Printf(format, "<measurement-uuid>", "measurement UUID")
		return nil
	}
	if len(args) != 1 {
		cliFatal("users services requires exactly one argument: <measurement-uuid>>")
	}
	if err := common.ValidateFormat(args, common.MeasurementUUID); err != nil {
		cliFatal(err)
	}
	return nil
}

func usersMeServices(cmd *cobra.Command, args []string) {
	uuid := args[0]
	url := fmt.Sprintf("%s/me/services?measurement_uuid=%v", common.UsersAPI, uuid)
	if _, err := common.Curl(auth.GetAccessToken(), false, "GET", url); err != nil {
		fatal(err)
	}
}

func getUsersMe(printOut bool) ([]byte, error) {
	url := fmt.Sprintf("%s/me", common.UsersAPI)
	return getUsers(url, printOut)
}

func getUsersAll(printOut bool) ([]byte, error) {
	url := fmt.Sprintf("%s?filter_verified=%v&offset=0&limit=200", common.UsersAPI, fAllVerified)
	return getUsers(url, printOut)
}

func deleteUsersById(userId string) error {
	url := fmt.Sprintf("%s/%v", common.UsersAPI, userId)
	jsonData, err := common.Curl(auth.GetAccessToken(), false, "DELETE", url)
	if err != nil {
		fmt.Println(string(jsonData))
		fatal(err)
	}
	return nil
}

func patchUsersId(userId, userFile string) error {
	fmt.Printf("users patch not implemented yet\n")
	return nil
}

func getUsers(url string, printOut bool) ([]byte, error) {
	jsonData, err := common.Curl(auth.GetAccessToken(), false, "GET", url)
	if err != nil {
		return jsonData, err
	}
	tmpFile, err := os.CreateTemp("/tmp", "irisctl-user-")
	if err != nil {
		return jsonData, err
	}
	defer tmpFile.Close()
	if common.RootFlagBool("no-delete") {
		fmt.Printf("saving in %s\n", tmpFile.Name())
	} else {
		defer func(f string) { verbose("removing %s\n", f); os.Remove(f) }(tmpFile.Name())
	}
	if _, err := tmpFile.Write(jsonData); err != nil {
		return jsonData, err
	}
	if printOut && !common.RootFlagBool("no-delete") {
		cmd := exec.Command("jq", ".", tmpFile.Name())
		jsonData, err = cmd.Output()
		if err != nil {
			return jsonData, err
		}
		fmt.Println(string(jsonData))
	}
	return jsonData, nil
}
