// Package auth implements all authentication APIs of Iris.
package auth

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/dioptra-io/irisctl/internal/common"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	// Command, its flags, subcommands, and their flags.
	//	auth <subcommand>
	//	auth login [--cookie]
	//	auth logout [--cookie]
	//	auth register <user-details>...
	cmdName       = "auth"
	subcmdNames   = []string{"login", "logout", "register"}
	fLoginCookie  bool
	fLogoutCookie bool

	// Errors.
	ErrNoAccessToken = errors.New("no access token")

	// Test code can change Fatal to Panic, allowing recovery
	// from a fatal error without causing the process to exit.
	fatal    = log.Fatal
	cliFatal = common.CliFatal
	verbose  = common.Verbose
)

// AuthCmd retuns the command structure for auth.
func AuthCmd() *cobra.Command {
	authCmd := &cobra.Command{
		Use:       cmdName,
		ValidArgs: subcmdNames,
		Short:     "authentication API commands",
		Long:      "authentication API commands for user login, logout, and registration",
		Args:      authArgs,
		Run:       auth,
	}
	authCmd.SetUsageFunc(common.Usage)
	authCmd.SetHelpFunc(common.Help)

	// auth login and its flags
	loginSubcmd := &cobra.Command{
		Use:   "login",
		Short: "login user",
		Long:  "authenticate user and login with either cookie or json web token (jwt)",
		Args:  authLoginArgs,
		Run:   authLogin,
	}
	loginSubcmd.Flags().BoolVar(&fLoginCookie, "cookie", false, "use cookie instead of json web token (jwt) to login")
	authCmd.AddCommand(loginSubcmd)

	// auth logout and its flags
	logoutSubcmd := &cobra.Command{
		Use:   "logout",
		Short: "logout user",
		Long:  "de-authenticate user and logout with either cookie or json web token (jwt)",
		Args:  authLogoutArgs,
		Run:   authLogout,
	}
	logoutSubcmd.Flags().BoolVar(&fLogoutCookie, "cookie", false, "use cookie instead of json web token (jwt) to logout")
	authCmd.AddCommand(logoutSubcmd)

	// auth register (has no flags)
	registerSubcmd := &cobra.Command{
		Use:   "register",
		Short: "register a user",
		Long:  "register a user whose details are in the specified file",
		Args:  authRegisterArgs,
		Run:   authRegister,
	}
	authCmd.AddCommand(registerSubcmd)

	return authCmd
}

// GetAccessToken returns an access token string.
func GetAccessToken() string {
	if common.RootFlagBool("no-auto-login") {
		verbose("skipping auto login because --no-auto-login is set\n")
		return ""
	}
	accessToken, err := postAuthLogin()
	if err != nil {
		fatal(err)
	}
	return accessToken
}

func authArgs(cmd *cobra.Command, args []string) error {
	if _, ok := common.IsUsage(args); ok {
		return nil
	}
	if len(args) == 0 {
		cliFatal("auth requires one of these subcommands: ", strings.Join(subcmdNames, " "))
	}
	cliFatal("unknown subcommand: ", args[0])
	return nil
}

func auth(cmd *cobra.Command, args []string) {
	fatal("auth()")
}

func authLoginArgs(cmd *cobra.Command, args []string) error {
	if _, ok := common.IsUsage(args); ok {
		return nil
	}
	if len(args) != 0 {
		cliFatal("auth login does not take any arguments")
	}
	return nil
}

func authLogin(cmd *cobra.Command, args []string) {
	if _, err := postAuthLogin(); err != nil {
		fatal(err)
	}
}

func authLogoutArgs(cmd *cobra.Command, args []string) error {
	if _, ok := common.IsUsage(args); ok {
		return nil
	}
	if len(args) != 0 {
		cliFatal("auth logout does not take any arguments")
	}
	return nil
}

func authLogout(cmd *cobra.Command, args []string) {
	if err := postAuthLogout(); err != nil {
		fatal(err)
	}
}

func authRegisterArgs(cmd *cobra.Command, args []string) error {
	if format, ok := common.IsUsage(args); ok {
		fmt.Printf(format, "<user-details>", "file containing user details in JSON format")
		return nil
	}
	if len(args) < 1 {
		cliFatal("auth register requires exactly one argument: <user-details>", common.UserFile)
	}
	return nil
}

func authRegister(cmd *cobra.Command, args []string) {
	for _, arg := range args {
		if err := postAuthRegister(arg); err != nil {
			fatal(err)
		}
	}
}

func postAuthLogin() (string, error) {
	if fLoginCookie {
		fmt.Printf("auth login --cookie not implemented yet\n")
		return "", nil
	}
	home := os.Getenv("HOME")
	if home == "" {
		return "", common.ErrHomeEnv
	}
	irisHome := fmt.Sprintf("%s/.iris", home)
	if err := os.MkdirAll(irisHome, 0700); err != nil {
		return "", err
	}

	credentialsFile := fmt.Sprintf("%s/credentials", irisHome)
	if _, err := common.CheckFile("credentials", credentialsFile); err != nil {
		return "", err
	}

	accessTokenFile := fmt.Sprintf("%s/jwt", irisHome)
	fi, err := common.CheckFile("access token", accessTokenFile)
	if errors.Is(err, os.ErrNotExist) {
		verbose("creating access token file %s\n", accessTokenFile)
		if err = createAccessToken(credentialsFile, accessTokenFile); err != nil {
			return "", err
		}
	} else {
		if !fi.Mode().IsRegular() {
			return "", common.ErrNotRegularFile
		}
		now := time.Now()
		oneHourAgo := now.Add(-time.Hour)
		if fi.ModTime().Before(oneHourAgo) {
			verbose("recreating access token file %s because it's too old\n", accessTokenFile)
			if err = createAccessToken(credentialsFile, accessTokenFile); err != nil {
				return "", err
			}
		} else {
			verbose("access token file exists and is current\n")
		}
	}

	contents, err := os.ReadFile(accessTokenFile)
	if err != nil {
		return "", err
	}
	return string(contents), nil
}

func postAuthLogout() error {
	fmt.Println("auth logout not implemented yet")
	return nil
}

func postAuthRegister(userFile string) error {
	fmt.Println("auth register not implemented yet, user file:", userFile)
	return nil
}

func createAccessToken(credentialsFile, accessTokenFile string) error {
	username, err := getIrisUser(credentialsFile)
	if err != nil {
		return err
	}
	password := os.Getenv("IRIS_PASSWORD")
	if password == "" {
		fmt.Fprintf(os.Stderr, "Enter password for Iris user %s: ", username)
		line, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return err
		}
		password = string(line)
	} else {
		fmt.Fprintf(os.Stderr, "using IRIS_PASSWORD environment variable\n")
	}

	url := fmt.Sprintf("%s/jwt/login", common.APIEndpoint(common.AuthAPISuffix))
	data := fmt.Sprintf("grant_type=&username=%s&password=%s&scope=&client_id=&client_secret=", username, password)
	jsonData, err := common.Curl("", false, "POST", url,
		"-H", "Content-Type: application/x-www-form-urlencoded",
		"-d", data,
	)
	if err != nil {
		fmt.Println(string(jsonData))
		return err
	}
	var responseData map[string]interface{}
	if err := json.Unmarshal(jsonData, &responseData); err != nil {
		return err
	}

	accessToken, ok := responseData["access_token"].(string)
	if !ok {
		verbose("access_token not found or not a string\n")
		return ErrNoAccessToken
	}
	return os.WriteFile(accessTokenFile, []byte(accessToken), 0600)
}

func getIrisUser(credentialsFile string) (string, error) {
	file, err := os.Open(credentialsFile)
	if err != nil {
		return "", err
	}
	fileScanner := bufio.NewScanner(file)
	fileScanner.Split(bufio.ScanLines)
	user := ""
	for fileScanner.Scan() {
		line := fileScanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		if user == "" {
			user = line
		}
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return user, nil
}
