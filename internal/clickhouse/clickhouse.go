package clickhouse

import (
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/dioptra-io/irisctl/internal/common"
	"github.com/dioptra-io/irisctl/internal/users"
	"github.com/spf13/cobra"
)

var (
	// Command, its flags, subcommands, and their flags.
	//      clickhouse --query <query-string>
	//      clickhouse <query-file>
	cmdName           = "clickhouse"
	subcmdNames       = []string{}
	fClickHouseQuery  string
	fClickhouseURL    string
	fClickhouseParams string

	// Test code changes Fatal to Panic so a fatal error won't exit
	// the process and can be recovered.
	fatal    = log.Fatal
	cliFatal = common.CliFatal
	verbose  = common.Verbose
)

// ClickHouseCmd returns the command structure for clickhouse.
func ClickHouseCmd() *cobra.Command {
	clickhouseCmd := &cobra.Command{
		Use:       cmdName,
		Short:     "clickhouse query",
		Long:      "clickhouse query",
		ValidArgs: subcmdNames,
		Args:      clickhouseArgs,
		Run:       clickhouse,
	}
	clickhouseCmd.Flags().StringVar(&fClickHouseQuery, "query", "", "clickhouse query string")
	clickhouseCmd.Flags().StringVar(&fClickhouseURL, "clickhouse-proxy-url", "https://chproxy.iris.dioptra.io", "proxy url of the clickhouse server")
	clickhouseCmd.Flags().StringVar(&fClickhouseParams, "clickhouse-params", "enable_http_compression=false&default_format=JSONEachRow&output_format_json_quote_64bit_integer", "raw string of clickhouse parameters")
	clickhouseCmd.SetUsageFunc(common.Usage)
	clickhouseCmd.SetHelpFunc(common.Help)

	return clickhouseCmd
}

func RunQueryString(query string) (string, string, error) {
	verbose("querying clickhouse with the query string %s\n", query)
	userpass, err := users.GetUserPass()
	if err != nil {
		return "", "", err
	}
	tmpFile, err := os.CreateTemp("/tmp", "irisctl-clickhouse-")
	if err != nil {
		return "", "", err
	}
	defer tmpFile.Close()
	url := fmt.Sprintf("%v/?%v&database=iris&query=%v", fClickhouseURL, fClickhouseParams, url.QueryEscape(query))
	output, err := common.Curl(userpass, true, "POST", url, "--http1.1", "--output", tmpFile.Name())
	return tmpFile.Name(), string(output), err
}

func clickhouseArgs(cmd *cobra.Command, args []string) error {
	if format, ok := common.IsUsage(args); ok {
		fmt.Printf(format, "<file>", "query-file")
		return nil
	}
	if len(args) > 1 {
		cliFatal("clickhouse takes at most one argument: <query-file>")
	}
	if fClickHouseQuery == "" && len(args) == 0 {
		cliFatal("specify either a query-string or a query-file")
	}
	if fClickHouseQuery != "" && len(args) > 0 {
		cliFatal("cannot use --query and also specify a query-file")
	}
	return nil
}

func clickhouse(cmd *cobra.Command, args []string) {
	var tmpFile, output string
	var err error

	if len(args) > 0 {
		tmpFile, output, err = runQueryFromFile(args[0])
	} else {
		tmpFile, output, err = RunQueryString(fClickHouseQuery)
	}
	if err != nil {
		fmt.Printf("%v\n", output)
		fatal(err)
	}
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("%v\n", string(content))
}

func runQueryFromFile(queryFile string) (string, string, error) {
	verbose("querying clickhouse with the query in %s\n", queryFile)
	content, err := os.ReadFile(queryFile)
	if err != nil {
		return "", "", err
	}
	query := string(content)
	return RunQueryString(query)
}
