// Package check implements commands for checking Iris.
package check

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"

	"github.com/dioptra-io/irisctl/internal/agents"
	"github.com/dioptra-io/irisctl/internal/common"
	"github.com/dioptra-io/irisctl/internal/users"
	"github.com/spf13/cobra"
)

const (
	uptimeCmd          = "uptime"
	netCmd             = "bash -c 'cat /sys/class/net/eth0/statistics/[rt]x_{bytes,packets}'"
	dockerAgentLogsCmd = "docker logs --timestamps iris-agent"
	dockerPsCmd        = "docker ps --format 'table {{.ID}}\\t{{.Names}}\\t{{.Status}}'"
)

var (
	// Command, its flags, subcommands, and their flags.
	//	check <subcommand>
	//	check agents [--uptime] [--net]
	//	check containers [--errors] [--logs] [<agent>...]
	//	check uuids [<meas-md-file>] <uuid>...
	cmdName          = "check"
	subcmdNames      = []string{"agents", "containers", "uuids"}
	fAgentUptime     bool
	fAgentNet        bool
	fContainerErrors bool
	fContainerLogs   bool

	// Test code changes Fatal to Panic so a fatal error won't exit
	// the process and can be recovered.
	fatal    = log.Fatal
	cliFatal = common.CliFatal
	verbose  = common.Verbose
)

// CheckCmd returns the command structure for check.
func CheckCmd() *cobra.Command {
	checkCmd := &cobra.Command{
		Use:       cmdName,
		ValidArgs: subcmdNames,
		Short:     "commands to check Iris",
		Long:      "commands to check Iris agents and containers",
		Args:      checkArgs,
		Run:       check,
	}
	checkCmd.SetUsageFunc(common.Usage)
	checkCmd.SetHelpFunc(common.Help)

	// check agents and its flags
	agentsSubcmd := &cobra.Command{
		Use:   "agents",
		Short: "show status of agent(s)",
		Long:  "show status of agent(s)",
		Args:  checkAgentsArgs,
		Run:   checkAgents,
	}
	agentsSubcmd.Flags().BoolVar(&fAgentUptime, "uptime", false, "show uptime")
	agentsSubcmd.Flags().BoolVar(&fAgentNet, "net", false, "show network bytes and packets sent and received")
	checkCmd.AddCommand(agentsSubcmd)

	// check containers and its flags
	containersSubcmd := &cobra.Command{
		Use:   "containers",
		Short: "show information about container(s)",
		Long:  "show information about container(s)",
		Args:  checkContainersArgs,
		Run:   checkContainers,
	}
	containersSubcmd.Flags().BoolVar(&fContainerErrors, "errors", false, "show errors in container logs")
	containersSubcmd.Flags().BoolVar(&fContainerLogs, "logs", false, "show container logs")
	checkCmd.AddCommand(containersSubcmd)

	// check uuids (has no flags)
	uuidsSubcmd := &cobra.Command{
		Use:   "uuids",
		Short: "show information about uuid(s)",
		Long:  "show information about uuid(s)",
		Args:  checkUuidsArgs,
		Run:   checkUuids,
	}
	checkCmd.AddCommand(uuidsSubcmd)

	return checkCmd
}

func checkArgs(cmd *cobra.Command, args []string) error {
	if _, ok := common.IsUsage(args); ok {
		return nil
	}
	if len(args) == 0 {
		cliFatal("check requires one of these subcommands: ", strings.Join(subcmdNames, " "))
	}
	cliFatal("unknown subcommand: ", args[0])
	return nil
}

func check(cmd *cobra.Command, args []string) {
	fatal("check()")
}

func checkAgentsArgs(cmd *cobra.Command, args []string) error {
	if _, ok := common.IsUsage(args); ok {
		return nil
	}
	if len(args) != 0 {
		cliFatal("check agents does not take any arguments")
	}
	return nil
}

func checkAgents(cmd *cobra.Command, args []string) {
	jsonData, err := agents.GetAgents("", false)
	if err != nil {
		fatal(err)
	}
	if err := printAgentsStatus(jsonData); err != nil {
		fatal(err)
	}
	if !fAgentUptime && !fAgentNet {
		return
	}
	gcpHostnames, err := common.ParseGCPHostnames(jsonData)
	if err != nil {
		fatal(err)
	}
	if fAgentUptime {
		verbose("getting agent uptimes takes a few seconds\n")
		fmt.Printf("%-30s   %-68s\n", "hostname", "uptime")
		if errs := agentDetails(gcpHostnames, "uptime"); errs != nil {
			fatal(errs)
		}
	}
	if fAgentNet {
		fmt.Printf("%-30s   %-12s  %-12s  %-10s  %-10s\n", "hostname", "rx_bytes", "tx_bytes", "rx_packets", "tx_packets")
		if errs := agentDetails(gcpHostnames, "net"); errs != nil {
			fatal(errs)
		}
	}
}

func checkContainersArgs(cmd *cobra.Command, args []string) error {
	if format, ok := common.IsUsage(args); ok {
		fmt.Printf(format, "<agent>...", "one or more agent UUIDs or hostnames")
		return nil
	}
	return nil
}

func checkContainers(cmd *cobra.Command, args []string) {
	var gcpHostnames []string
	if len(args) > 0 {
		gcpHostnames = args
	} else {
		jsonData, err := agents.GetAgents("", false)
		if err != nil {
			fatal(err)
		}
		gcpHostnames, err = common.ParseGCPHostnames(jsonData)
		if err != nil {
			fatal(err)
		}
	}
	verbose("checking agent container logs of %v\n", gcpHostnames)
	if err := checkContainersAgent(gcpHostnames); err != nil {
		fatal(err)
	}
}

func checkUuidsArgs(cmd *cobra.Command, args []string) error {
	if format, ok := common.IsUsage(args); ok {
		fmt.Printf(format, "<uuids>...", "one or UUIDs")
		return nil
	}
	if len(args) == 0 {
		cliFatal("check uuids requires at least one argument: <uuid>...")
	}
	n := 0
	if err := common.ValidateFormat([]string{args[0]}, common.UserID); err != nil {
		if len(args) < 1 {
			cliFatal("check uuids requires at least one argument: <uuid>...")
		}
		_, err := common.CheckFile("meas-md-file", args[0])
		if err != nil {
			cliFatal(err)
		}
		n = 1
	}
	if err := common.ValidateFormat(args[n:], common.UserID); err != nil {
		cliFatal(err)
	}
	return nil
}

func checkUuids(cmd *cobra.Command, args []string) {
	n := 0
	if err := common.ValidateFormat([]string{args[0]}, common.UserID); err != nil {
		n = 1
		_, err := common.CheckFile("meas-md-file", args[0])
		if err != nil {
			fatal(err)
		}
	}

	jsonData, err := users.GetUserUUIDs()
	if err != nil {
		fatal(err)
	}
	var users common.Users
	if err := json.Unmarshal(jsonData, &users); err != nil {
		fatal(err)
	}
	for _, arg := range args[n:] {
		found := false
		fmt.Printf("%v ", arg)
		for _, user := range users.Results {
			if arg == user.UUID {
				found = true
				fmt.Printf("%v %v %v\n", user.FirstName, user.LastName, user.Email)
				break
			}
		}
		if !found {
			fmt.Printf("?\n")
		}
	}
}

func checkContainersAgent(gcpHostnames []string) []error {
	if !fContainerErrors && !fContainerLogs {
		if errs := agentDetails(gcpHostnames, "dockerps"); errs != nil {
			return errs
		}
	}
	if fContainerErrors {
		if errs := agentDetails(gcpHostnames, "errors"); errs != nil {
			return errs
		}
	}
	if fContainerLogs {
		if errs := agentDetails(gcpHostnames, "logs"); errs != nil {
			return errs
		}
	}
	return nil
}

func printAgentsStatus(jsonData []byte) error {
	// For a single hostname:
	// filter := []string{"-r", "\"\\(.uuid) \\(.state) \\(.parameters.hostname) \\(.parameters.version)\""}
	filter := []string{"-r", ".results[] | \"\\(.uuid) \\(.state) \\(.parameters.hostname) \\(.parameters.version)\""}
	jqOutput, err := common.JqBytes(jsonData, filter)
	if err != nil {
		fatal(err)
	}

	cmd := exec.Command("awk", "{ printf(\"%s  %-10s  %-24s  %s\\n\",  $1, $2, $3, $4) }")
	cmd.Stdin = bytes.NewBuffer(jqOutput)
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	return err
}

func agentDetails(gcpHostnames []string, what string) []error {
	var remoteCmd string
	switch what {
	case "uptime":
		remoteCmd = uptimeCmd
	case "net":
		remoteCmd = netCmd
	case "dockerps":
		remoteCmd = dockerPsCmd
	case "errors":
		remoteCmd = dockerAgentLogsCmd
	case "logs":
		remoteCmd = dockerAgentLogsCmd
	default:
		fatal(what)
	}
	var wg sync.WaitGroup
	allOutput := make(chan []string, len(gcpHostnames))
	allErrors := make(chan error, len(gcpHostnames))
	for _, hostname := range gcpHostnames {
		verbose("checking agent %v\n", hostname)
		wg.Add(1)
		go func(hostname string) {
			defer wg.Done()
			output, err := common.GcloudSSH(hostname, remoteCmd)
			if err != nil {
				allErrors <- fmt.Errorf("%s: %v", hostname, err)
				return
			}
			allOutput <- output
		}(hostname)
	}
	wg.Wait()
	close(allOutput)
	close(allErrors)
	for output := range allOutput {
		s := []string{}
		for i, o := range output {
			o = strings.TrimRight(o, "\r")
			if strings.HasPrefix(o, "Connection to ") || strings.HasPrefix(o, "CONTAINER ID") {
				continue
			}
			switch what {
			case "uptime":
				fallthrough
			case "net":
				o = strings.TrimRight(o, "\r\n")
				if i == 0 {
					s = append(s, fmt.Sprintf("%-30s", o)) // hostname that we wrote to channel
				} else {
					s = append(s, o, "  ") // output lines of the command
				}
			case "dockerps":
				s = append(s, o)
			case "errors":
				if strings.Contains(strings.ToLower(o), "error") {
					s = append(s, agents.ReplaceAgentUUIDs(o))
				}
			case "logs":
				s = append(s, o)
			default:
				fatal(what)
			}
		}
		if len(s) > 0 {
			fmt.Println(strings.Join(s, "  "))
		}
	}
	var errors []error
	for err := range allErrors {
		if err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}
