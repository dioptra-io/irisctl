// Package agents implements all agent APIs of Iris.
package agents

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
	//	agents [--tag]
	//	agents [<agent>...]
	cmdName     = "agents"
	subcmdNames = []string{}
	fAgentsTag  string

	agentsUUIDName = make(map[string]string)

	// Test code changes Fatal to Panic so a fatal error won't exit
	// the process and can be recovered.
	fatal    = log.Fatal
	cliFatal = common.CliFatal
	verbose  = common.Verbose
)

// AgentsCmd returns the command structure for agents.
func AgentsCmd() *cobra.Command {
	agentsCmd := &cobra.Command{
		Use:       cmdName,
		ValidArgs: subcmdNames,
		Short:     "agents API commands",
		Long:      "agents API commands for getting all or specific agents",
		Args:      agentsArgs,
		Run:       agents,
	}
	agentsCmd.Flags().StringVar(&fAgentsTag, "tag", "", "get only agents that have the specified tag")
	agentsCmd.SetUsageFunc(common.Usage)
	agentsCmd.SetHelpFunc(common.Help)

	return agentsCmd
}

func GetAgentName(uuid string) string {
	name, ok := agentsUUIDName[uuid]
	if !ok {
		return "?"
	}
	return name
}

func GetAgents(hostname string, printOut bool) ([]byte, error) {
	var url string
	if fAgentsTag != "" {
		url = fmt.Sprintf("%s/?tag=%v&offset=0&limit=200", common.AgentsAPI, fAgentsTag)
	} else {
		url = fmt.Sprintf("%s/?&offset=0&limit=200", common.AgentsAPI)
	}
	return getResults(url, hostname, printOut)
}

func ReplaceAgentUUIDs(s string) string {
	for uuid, hostname := range agentsUUIDName {
		s = strings.ReplaceAll(s, uuid, hostname)
	}
	return s
}

func agentsArgs(cmd *cobra.Command, args []string) error {
	if format, ok := common.IsUsage(args); ok {
		fmt.Printf(format, "<agent>...", "one or more agent UUIDs or hostnames")
	}
	return nil
}

func agents(cmd *cobra.Command, args []string) {
	if fAgentsTag != "" || len(args) == 0 {
		if len(args) != 0 {
			cliFatal("cannot use --tag and also specify an agent uuid")
		}
		if _, err := GetAgents("", !common.RootFlagBool("curl")); err != nil {
			fatal(err)
		}
		return
	}
	for _, arg := range args {
		if strings.Contains(arg, "iris") {
			if _, err := GetAgents(arg, !common.RootFlagBool("curl")); err != nil {
				fatal(err)
			}
		} else {
			if err := getAgentByUUID(arg); err != nil {
				fatal(err)
			}
		}
	}
}

func getAgentByUUID(uuid string) error {
	url := fmt.Sprintf("%s/%s", common.AgentsAPI, uuid)
	_, err := getResults(url, "", true)
	return err
}

func getResults(url, hostname string, printOut bool) ([]byte, error) {
	jsonData, err := common.Curl(auth.GetAccessToken(), false, "GET", url)
	if err != nil {
		fmt.Println(string(jsonData))
		return nil, err
	}
	file, err := common.WriteResults("irisctl-agents", jsonData)
	if !common.RootFlagBool("no-delete") {
		defer func(f string) { verbose("removing %s\n", f); os.Remove(f) }(file)
	}
	if err != nil {
		return jsonData, err
	}
	if printOut {
		var filter []string
		if hostname != "" {
			filter = append(filter, fmt.Sprintf(".results[] | select(.parameters.hostname == \"%s\")", hostname))
		} else {
			filter = append(filter, ".")
		}
		var jqOutput []byte
		jqOutput, err = common.JqBytes(jsonData, filter)
		fmt.Println(string(jqOutput))
	}
	return jsonData, err
}
