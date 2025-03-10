// Package analyze implements commands for analysis of Iris measurement
// data (not in the Iris API).
package analyze

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"gonum.org/v1/gonum/stat"

	"github.com/dioptra-io/irisctl/internal/agents"
	"github.com/dioptra-io/irisctl/internal/common"
	"github.com/dioptra-io/irisctl/internal/meas"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	FirstMeasurementDate = "2020-01-01T00:00:00.000"
	LastMeasurementDate  = "2040-01-01T00:00:00.000" // distant future

	DurationNone    = 1
	DurationTooLong = 2
	DurationOK      = 3
)

type tableDetails struct {
	modTime string
	rows    int
	bytes   int
}

var (
	// Command, its flags, subcommands, and their flags.
	//      analyze [--all-users] [--before <yyyy-mm-ddThh:mm:ss>] [--after <yyyy-mm-ddThh:mm:ss>] [--state <state>]... [--tag <tag>]... [--tags-and] [--agent <agent-hostname>]...
	//      analyze hours [--chart]
	//      analyze tags
	//      analyze states
	//      analyze tables [--meas-uuid <meas-uuid>] <meas-md-file>
	cmdName          = "analyze"
	subcmdNames      = []string{"hours", "tags", "states", "tables"}
	fAnalyzeAllUsers bool
	fAnalyzeBefore   common.CustomTime
	fAnalyzeAfter    common.CustomTime
	fAnalyzeState    []string
	fAnalyzeTag      []string
	fAnalyzeTagsAnd  bool
	fAnalyzeAgents   []string
	fHoursChart      bool
	fTablesMeasUUID  string

	// Errors.
	ErrInvalidTableName = errors.New("invalid table name")

	// Test code changes Fatal to Panic so a fatal error won't exit
	// the process and can be recovered.
	fatal    = log.Fatal
	cliFatal = common.CliFatal
	verbose  = common.Verbose

	hours = []string{
		"00", "01", "02", "03", "04", "05", "06", "07",
		"08", "09", "10", "11", "12", "13", "14", "15",
		"16", "17", "18", "19", "20", "21", "22", "23",
	}

	totFound        = 0
	totAgentFailure = 0
	totCanceled     = 0
	totFinished     = 0
	totOngoing      = 0
	nResults        = 0
	durationCS      = []float64{}
	durationSE      = []float64{}
	agentsPerMeas   = make(map[int]int)
	abbrState       = map[string]string{
		"agent_failure": "E",
		"canceled":      "C",
		"finished":      "F",
		"ongoing":       "O",
	}
)

func init() {
	if err := fAnalyzeAfter.Set(FirstMeasurementDate); err != nil {
		fatal(err)
	}
	if err := fAnalyzeBefore.Set(LastMeasurementDate); err != nil {
		fatal(err)
	}
}

// AnalyzeCmd returns the command structure for analyze.
func AnalyzeCmd() *cobra.Command {
	analyzeCmd := &cobra.Command{
		Use:       cmdName,
		Short:     "analyze commands",
		Long:      "analyze the metadata of measurements in the specified file",
		ValidArgs: subcmdNames,
		Args:      analyzeArgs,
		Run:       analyze,
	}
	analyzeCmd.Flags().BoolVar(&fAnalyzeAllUsers, "all-users", false, "match all measurements of all users (admin only)")
	analyzeCmd.Flags().Var(&fAnalyzeBefore, "before", "match measurements before the specified date (exclusive)")
	analyzeCmd.Flags().Var(&fAnalyzeAfter, "after", "match measurements after the specified date (inclusive)")
	analyzeCmd.Flags().StringArrayVarP(&fAnalyzeState, "state", "s", []string{}, "repeatable: match measurements with the specified state (agent_failure, canceled, finished, ongoing)")
	analyzeCmd.Flags().StringArrayVarP(&fAnalyzeTag, "tag", "t", []string{}, "repeatable: match measurements with the specified tag (also see --tags-and)")
	analyzeCmd.Flags().BoolVar(&fAnalyzeTagsAnd, "tags-and", false, "match measurements that have all specified tags")
	analyzeCmd.Flags().StringArrayVarP(&fAnalyzeAgents, "agent", "a", []string{}, "repeatable: match measurements that ran on the specified agent")
	analyzeCmd.SetUsageFunc(common.Usage)
	analyzeCmd.SetHelpFunc(common.Help)

	// analyze hours and its flags
	hoursCmd := &cobra.Command{
		Use:   "hours",
		Short: "show the number of measurements per hour",
		Long:  "show the number of measurements per hour per day in a dot chart",
		Args:  analyzeHoursArgs,
		Run:   analyzeHours,
	}
	hoursCmd.Flags().BoolVar(&fHoursChart, "chart", false, "create a dot chart file")
	analyzeCmd.AddCommand(hoursCmd)

	// analyze tags and its flags
	tagsCmd := &cobra.Command{
		Use:   "tags",
		Short: "analyze tags",
		Long:  "analyze the tags of measurement runs",
		Args:  analyzeTagsArgs,
		Run:   analyzeTags,
	}
	analyzeCmd.AddCommand(tagsCmd)

	// analyze states and its flags
	statesCmd := &cobra.Command{
		Use:   "states",
		Short: "analyze states",
		Long:  "analyze the states of measurement runs",
		Args:  analyzeStatesArgs,
		Run:   analyzeStates,
	}
	analyzeCmd.AddCommand(statesCmd)

	// analyze tables and its flags
	tablesSubcmd := &cobra.Command{
		Use:   "tables",
		Short: "list measurement tables",
		Long:  "list all tables created for each measurement",
		Args:  analyzeTablesArgs,
		Run:   analyzeTables,
	}
	tablesSubcmd.Flags().StringVar(&fTablesMeasUUID, "meas-uuid", "", "measurement UUID")
	analyzeCmd.AddCommand(tablesSubcmd)

	return analyzeCmd
}

func analyzeArgs(cmd *cobra.Command, args []string) error {
	if format, ok := common.IsUsage(args); ok {
		fmt.Printf(format, "<meas-md-file>", "optional: measurements metadata file")
		return nil
	}
	if len(args) > 1 {
		cliFatal("analyze takes at most one argument: <meas-md-file>")
	}
	validateFlags()
	if fAnalyzeAllUsers && len(args) > 0 {
		fmt.Printf("WARNING: ignoring --all-users because a measurement metadata file is specidfied\n")
		fAnalyzeAllUsers = false
	}
	return nil
}

func analyze(cmd *cobra.Command, args []string) {
	measurements, err := getMeasurements(args)
	if err != nil {
		fatal(err)
	}
	for _, measurement := range measurements {
		if measSkip(measurement) {
			continue
		}
		duration := measDuration(measurement) // may print WARNING or INFO
		if duration == DurationNone {
			continue
		}
		issues := []string{}
		totFound++
		if duration == DurationTooLong {
			issues = append(issues, "took too long")
		}
		measState(measurement.State)             // does not print anything
		if measAgents(measurement.Agents) == 0 { // does not print anything
			issues = append(issues, "has no agents")
		}
		printMeasDetails(measurement, issues)
	}
	printAnalysis("all")
}

func analyzeHoursArgs(cmd *cobra.Command, args []string) error {
	if format, ok := common.IsUsage(args); ok {
		fmt.Printf(format, "<mead-md-file>", "optional: measurements metadata file")
		return nil
	}
	if len(args) > 1 {
		cliFatal("analyze hours takes at most one argument: <meas-md-file>")
	}
	validateFlags()
	return nil
}

func analyzeHours(cmd *cobra.Command, args []string) {
	measurements, err := getMeasurements(args)
	if err != nil {
		fatal(err)
	}
	measPerHourUntrimmed := make(map[string]map[string]int)
	if err := initHoursTable(measPerHourUntrimmed); err != nil {
		fatal(err)
	}
	for _, measurement := range measurements {
		if measSkip(measurement) {
			continue
		}
		d := measurement.CreationTime.Format("2006-01-02")
		if measPerHourUntrimmed[d] == nil {
			panic("analyzeHours")
		}
		t := fmt.Sprintf("%02d", measurement.CreationTime.Hour())
		measPerHourUntrimmed[d][t]++
	}

	// Find the first date that has a measurement.
	var sortedDates []string
	for date := range measPerHourUntrimmed {
		sortedDates = append(sortedDates, date)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(sortedDates)))
	firstDate := ""
	for _, date := range sortedDates {
		for _, hour := range hours {
			number, exists := measPerHourUntrimmed[date][hour]
			if exists && number > 0 {
				firstDate = date
				break
			}
		}
	}
	// Trim the map.
	measPerHour := make(map[string]map[string]int)
	for _, date := range sortedDates {
		measPerHour[date] = make(map[string]int)
		for _, hour := range hours {
			measPerHour[date][hour] = measPerHourUntrimmed[date][hour]
		}
		if date == firstDate {
			break
		}
	}

	if fHoursChart {
		if err := dotChart(measPerHour); err != nil {
			fatal(err)
		}
	} else {
		if err := textChart(measPerHour, sortedDates); err != nil {
			fatal(err)
		}
	}
}

func analyzeTagsArgs(cmd *cobra.Command, args []string) error {
	if format, ok := common.IsUsage(args); ok {
		fmt.Printf(format, "<meas-md-file>", "optional: measurements metadata file")
		return nil
	}
	if len(args) > 1 {
		cliFatal("analyze tags hours takes at most one argument: <meas-md-file>")
	}
	validateFlags()
	return nil
}

func analyzeTags(cmd *cobra.Command, args []string) {
	measurements, err := getMeasurements(args)
	if err != nil {
		fatal(err)
	}

	measTags := make(map[string]int)
	tagCounts := make(map[string]int)
	measTags["<no-tags>"] = 0
	for _, measurement := range measurements {
		if measSkip(measurement) {
			continue
		}
		if len(measurement.Tags) == 0 {
			measTags[""] = measTags[""] + 1
		} else {
			for _, tag := range measurement.Tags {
				count, ok := measTags[tag]
				if !ok {
					measTags[tag] = 1
				} else {
					measTags[tag] = count + 1
				}
			}
		}

		sortedTags := make([]string, len(measurement.Tags))
		copy(sortedTags, measurement.Tags)
		sort.Strings(sortedTags)
		tagStr := strings.Join(sortedTags, ",")
		tagCounts[tagStr]++
	}

	keys := make([]string, 0, len(measTags))
	for key := range measTags {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		return measTags[keys[i]] > measTags[keys[j]]
	})
	fmt.Printf("Count Tag\n")
	for _, k := range keys {
		fmt.Printf("%5d %q\n", measTags[k], k)
	}

	// Convert map to slice for sorting.
	tagCountsSlice := make([]struct {
		Tags  []string
		Count int
	}, 0, len(tagCounts))
	for tags, count := range tagCounts {
		tagCountsSlice = append(tagCountsSlice, struct {
			Tags  []string
			Count int
		}{strings.Split(tags, ","), count})
	}
	sort.Slice(tagCountsSlice, func(i, j int) bool {
		return tagCountsSlice[i].Count > tagCountsSlice[j].Count
	})
	fmt.Printf("\nCount Tags\n")
	for _, tc := range tagCountsSlice {
		fmt.Printf("%5d %s\n", tc.Count, tc.Tags)
	}
}

func analyzeStatesArgs(cmd *cobra.Command, args []string) error {
	if format, ok := common.IsUsage(args); ok {
		fmt.Printf(format, "<meas-md-file>", "optional: measurements metadata file")
		return nil
	}
	if len(args) > 1 {
		cliFatal("analyze states hours takes at most one argument: <meas-md-file>")
	}
	validateFlags()
	return nil
}

func analyzeStates(cmd *cobra.Command, args []string) {
	printAnalysis("states")
	measurements, err := getMeasurements(args)
	if err != nil {
		fatal(err)
	}
	for _, measurement := range measurements {
		if measSkip(measurement) {
			continue
		}
		totFound++
		measState(measurement.State)
	}
}

func analyzeTablesArgs(cmd *cobra.Command, args []string) error {
	if format, ok := common.IsUsage(args); ok {
		fmt.Printf(format, "<meas-md-file>", "optional: measurements metadata file")
		return nil
	}
	if len(args) > 1 {
		cliFatal("analyze tables takes at most one argument: <meas-md-file>")
	}
	validateFlags()
	return nil
}

// There are three different command line invocations to analyze measurement tables:
//  1. Analyze all tables without any filters (i.e., not looking for
//     specific tags, states, or dates). This is the fastest/easiest
//     way because we can get the information with one ClickHouse
//     query but because we do not know the details of individual
//     measurements we cannot check if all tables for a particular
//     measurement exist or not.
//  2. Analyze all or a subset of tables in the specified measurement
//     metadata file.  In this case, we already have a measMdFile and do
//     not need to create it by querying Iris API.
//  3. Analyze all or a subset of tables without a measurement
//     metadata file.  In this case, we first need to create a measMdFile.
func analyzeTables(cmd *cobra.Command, args []string) {
	verbose("analyze tables of measurement(s)\n")

	// Handle case 1.
	ta, _ := time.Parse("2006-01-02", FirstMeasurementDate)
	tb, _ := time.Parse("2006-01-02", LastMeasurementDate)
	if fAnalyzeAllUsers && len(args) == 0 && len(fAnalyzeTag) == 0 && len(fAnalyzeState) == 0 &&
		fTablesMeasUUID == "" && ta.Equal(fAnalyzeAfter.Time) && tb.Equal(fAnalyzeBefore.Time) {
		if err := analyzeTablesByName(); err != nil {
			fatal(err)
		}
		return
	}

	// Handle cases 2 and 3.
	measurements, err := getMeasurements(args)
	if err != nil {
		fatal(err)
	}
	n, err := analyzeTablesByMeasurement(measurements)
	if err != nil {
		fatal(err)
	}
	if n == 0 {
		fmt.Printf("no measurements; did you forget --all-users?\n")
	}
}

func analyzeTablesByName() error {
	measTables, err := getAllMeasTables()
	if err != nil {
		return err
	}
	return printTables(measTables)
}

func analyzeTablesByMeasurement(measurements []common.Measurement) (int, error) {
	n := 0
	for _, measurement := range measurements {
		if measSkip(measurement) || (fTablesMeasUUID != "" && fTablesMeasUUID != measurement.UUID) {
			verbose("skipping %v\n", measurement.UUID)
			continue
		}
		if len(measurement.Agents) == 0 {
			verbose("skipping %v because it has 0 agents\n", measurement.UUID)
			continue
		}
		measTables, err := getOneMeasTables(measurement.UUID)
		if err != nil {
			if !errors.Is(err, common.ErrZeroLength) {
				return n, err
			}
			fmt.Printf("WARNING: no ClickHouse tables for measurement %v\n", measurement.UUID)
			continue
		}
		n++
		// Each measurement produces four tables: results_, prefixes_, links_, and _probes.
		nFound := len(measTables)
		output := fmt.Sprintf("%v [tags: %v] [state: %v] %d tables", measurement.UUID, strings.Join(measurement.Tags, ","), measurement.State, nFound)
		nExpected := len(measurement.Agents) * 4
		if nFound != nExpected {
			output = fmt.Sprintf("%s <== ERROR: expected %d", output, nExpected)
		}
		if viper.GetBool("verbose") {
			fmt.Println(output)
		} else {
			fmt.Printf("%d %s", n, measurement.UUID)
			if fTablesMeasUUID != "" {
				fmt.Printf("\n")
			} else {
				fmt.Printf("\r")
			}
		}
		if err := printTables(measTables); err != nil {
			return n, err
		}
	}
	if !viper.GetBool("verbose") {
		fmt.Println()
	}
	return n, nil
}

func printTables(measTables []MeasTable) error {
	data := map[string]tableDetails{}
	prevMeasUUID := ""
	for _, table := range measTables {
		modTime, err := time.Parse("2006-01-02 15:04:05", table.ModTime)
		if err != nil {
			return err
		}
		if !modTime.After(fAnalyzeAfter.Time) || !modTime.Before(fAnalyzeBefore.Time) {
			verbose("skipping %v\n", table.Name)
			continue
		}
		measUUID, _, err := parseMeasAgentUUIDs(table.Name)
		if err != nil {
			return err
		}
		if prevMeasUUID == "" {
			prevMeasUUID = measUUID
		}
		if prevMeasUUID != measUUID {
			fmt.Printf("%v\n", prevMeasUUID)
			printTableDetails(data)
			data = map[string]tableDetails{}
			prevMeasUUID = measUUID
		}
		data[table.Name] = tableDetails{
			modTime: table.ModTime,
			rows:    table.Rows,
			bytes:   table.Bytes,
		}
	}
	printTableDetails(data)
	return nil
}

func printTableDetails(data map[string]tableDetails) {
	for _, tblName := range sortByKey(data) {
		_, agentUUID, err := parseMeasAgentUUIDs(tblName)
		if err != nil {
			panic(err) // cannot happen
		}
		h := agents.GetAgentName(strings.ReplaceAll(agentUUID, "_", "-"))
		skip := false
		if len(fAnalyzeAgents) > 0 {
			skip = true
			for _, a := range fAnalyzeAgents {
				if a == h {
					skip = false
					break
				}
			}
		}
		if skip {
			continue
		}
		tblDetails := data[tblName]
		output := fmt.Sprintf("    %-86s %s %10s %10s  %s", tblName, tblDetails.modTime, common.HumanReadable(tblDetails.rows), common.HumanReadable(tblDetails.bytes), h)
		if tblDetails.rows == 0 || tblDetails.bytes == 0 {
			output = fmt.Sprintf("%s <== WARNING: expected > 0", output)
		}
		//output += "\n"
		fmt.Println(output)
	}
}

func sortByKey(data map[string]tableDetails) []string {
	var keys []string
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func parseMeasAgentUUIDs(tableName string) (string, string, error) {
	start := strings.Index(tableName, "__")
	if start == -1 {
		return "", "", ErrInvalidTableName
	}
	start += 2
	end := strings.Index(tableName[start:], "__")
	if end == -1 {
		return "", "", ErrInvalidTableName
	}
	measUUID := strings.ReplaceAll(tableName[start:start+end], "_", "-")
	end += start + 2
	agentUUID := tableName[end:]
	return measUUID, agentUUID, nil
}

func textChart(measPerHour map[string]map[string]int, sortedDates []string) error {
	// Print daily runs from the newest to the oldest.
	fmt.Printf("           ")
	for _, hour := range hours {
		fmt.Printf("%s ", hour)
	}
	fmt.Println()
	for _, date := range sortedDates {
		// Any runs on this day?
		if !viper.GetBool("verbose") {
			found := false
			for _, hour := range hours {
				number, exists := measPerHour[date][hour]
				if exists && number != 0 {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		fmt.Printf("%s ", date)
		for _, hour := range hours {
			number, exists := measPerHour[date][hour]
			if !exists || number == 0 {
				fmt.Printf(" . ")
				continue
			}
			fmt.Printf("%2d ", number)
		}
		fmt.Println()
	}
	return nil
}

func printMeasDetails(measurement common.Measurement, issues []string) {
	if !viper.GetBool("verbose") && len(issues) == 0 {
		return
	}
	c := time.Time(measurement.CreationTime.Time)
	s := time.Time(measurement.StartTime.Time)
	e := time.Time(measurement.EndTime.Time)
	a, ok := abbrState[measurement.State]
	if !ok {
		panic("internal error: invalid measurement state")
	}
	fmt.Printf("%4d %s %2d %s  ", totFound, measurement.UUID, len(measurement.Agents), a)
	fmt.Printf("%s   ", c.Format("06-01-02.15:04:05"))
	fmt.Printf("%s %3.fs  ", s.Format("06-01-02.15:04:05"), s.Sub(c).Seconds())
	fmt.Printf("%s %10s  ", e.Format("06-01-02.15:04:05"), e.Sub(s).Round(time.Second))
	fmt.Printf("%q", measurement.Tags)
	if len(issues) > 0 {
		fmt.Printf(" <== WARNING: %v", strings.Join(issues, ","))
	}
	fmt.Println()
}

func printAnalysis(what string) {
	if totFound == 0 {
		fmt.Printf("nothing to print\n")
		return
	}
	verbose("\n")

	// Tags.
	if what == "all" || what == "tags" {
		if len(fAnalyzeTag) > 0 {
			fmt.Printf("TAGS\n")
			fmt.Printf("    %s\n", fAnalyzeTag)
		}
	}

	// States.
	if what == "all" || what == "states" {
		fmt.Printf("STATES\n    total agent_failure canceled finished ongoing\n")
		fmt.Printf("    %5d %13d %8d %8d %7d\n", totFound, totAgentFailure, totCanceled, totFinished, totOngoing)
	}

	// Durations.
	if what == "all" || what == "durations" {
		fmt.Printf("DURATION\n")
		fmt.Printf("    %-10s %-12s %-12s %-12s %-12s\n", "Minimum", "Maximum", "Average", "Median (P50)", "P90")

		sort.Float64s(durationCS)
		mind := time.Duration(durationCS[0] * float64(time.Second))
		min := fmt.Sprintf("%v", mind.Round(time.Second))
		maxd := time.Duration(durationCS[len(durationCS)-1] * float64(time.Second))
		max := fmt.Sprintf("%v", maxd.Round(time.Second))
		avgd := time.Duration(stat.Mean(durationCS, nil) * float64(time.Second))
		avg := fmt.Sprintf("%v", avgd.Round(time.Second))
		p50 := stat.Quantile(0.5, stat.Empirical, durationCS, nil)
		p50d := time.Duration(p50 * float64(time.Second))
		p50s := fmt.Sprintf("%v", p50d.Round(time.Second))
		p90 := stat.Quantile(0.9, stat.Empirical, durationCS, nil)
		p90d := time.Duration(p90 * float64(time.Second))
		p90s := fmt.Sprintf("%v", p90d.Round(time.Second))
		fmt.Printf("    %-10s %-12s %-12s %-12s %-12s", min, max, avg, p50s, p90s)
		fmt.Printf("    creation time to start time\n")

		sort.Float64s(durationSE)
		mind = time.Duration(durationSE[0] * float64(time.Second))
		min = fmt.Sprintf("%v", mind.Round(time.Second))
		maxd = time.Duration(durationSE[len(durationSE)-1] * float64(time.Second))
		max = fmt.Sprintf("%v", maxd.Round(time.Second))
		avgd = time.Duration(stat.Mean(durationSE, nil) * float64(time.Second))
		avg = fmt.Sprintf("%v", avgd.Round(time.Second))
		p50 = stat.Quantile(0.5, stat.Empirical, durationSE, nil)
		p50d = time.Duration(p50 * float64(time.Second))
		p50s = fmt.Sprintf("%v", p50d.Round(time.Second))
		p90 = stat.Quantile(0.9, stat.Empirical, durationSE, nil)
		p90d = time.Duration(p90 * float64(time.Second))
		p90s = fmt.Sprintf("%v", p90d.Round(time.Second))
		fmt.Printf("    %-10s %-12s %-12s %-12s %-12s", min, max, avg, p50s, p90s)
		fmt.Printf("    start time to end time\n")
	}

	// Agents.
	if what == "all" || what == "agents" {
		keys := make([]int, 0, len(agentsPerMeas))
		for k := range agentsPerMeas {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		fmt.Printf("AGENTS PER MEASUREMENT\n")
		fmt.Printf("    Agents   Measurements\n")
		for _, k := range keys {
			fmt.Printf("    %-6d   %12d\n", k, agentsPerMeas[k])
		}
		fmt.Printf("These measurements should correspond to %d `results_*` tables in ClickHouse.\n", nResults)
	}
}

func initHoursTable(measPerHour map[string]map[string]int) error {
	currentDate := time.Now()
	startDate, err := time.Parse("2006-01-02", FirstMeasurementDate)
	if err != nil {
		return fmt.Errorf("failed to parse start date: %s", err)
	}
	for current := startDate; !current.After(currentDate); current = current.AddDate(0, 0, 1) {
		dateStr := current.Format("2006-01-02")
		for _, hour := range hours {
			measPerHour[dateStr] = make(map[string]int)
			measPerHour[dateStr][hour] = 0
		}
	}
	return nil
}

func measSkip(measurement common.Measurement) bool {
	if len(fAnalyzeTag) > 0 && !common.MatchTag(measurement.Tags, fAnalyzeTag, fAnalyzeTagsAnd) {
		return true
	}
	if len(fAnalyzeState) > 0 && !common.MatchState(measurement.State, fAnalyzeState) {
		return true
	}
	if !measurement.CreationTime.After(fAnalyzeAfter.Time) ||
		!measurement.CreationTime.Before(fAnalyzeBefore.Time) {
		return true
	}
	return false
}

func measState(state string) {
	switch state {
	case "agent_failure":
		totAgentFailure++
	case "canceled":
		totCanceled++
	case "finished":
		totFinished++
	case "ongoing":
		totOngoing++
	default:
		fatal("unknown state: ", state)
	}
}

func measAgents(agents []common.Agent) int {
	nAgents := len(agents)
	nResults += nAgents
	agentsPerMeas[nAgents]++
	return nAgents
}

func measDuration(measurement common.Measurement) int {
	c := time.Time(measurement.CreationTime.Time)
	if c.Year() == 1 && c.Month() == 1 && c.Day() == 1 {
		fmt.Printf("WARNING: skipping %s due to uninitialized creation time -- internal error?!\n", measurement.UUID)
		return DurationNone
	}
	s := time.Time(measurement.StartTime.Time)
	if s.Year() == 1 && s.Month() == 1 && s.Day() == 1 {
		fmt.Printf("WARNING: skipping %s due to uninitialized start time -- created at %v, waiting to start\n", measurement.UUID, c)
		return DurationNone
	}
	e := time.Time(measurement.EndTime.Time)
	if e.Year() == 1 && e.Month() == 1 && e.Day() == 1 {
		fmt.Printf("WARNING: skipping %s due to uninitialized end time -- started at %v, waiting to end\n", measurement.UUID, s)
		return DurationNone
	}
	durationCS = append(durationCS, float64(s.Sub(c).Seconds()))
	durationSE = append(durationSE, float64(e.Sub(s).Seconds()))
	expectedDuration := []time.Duration{5, 24} // TODO: Provide command line flags to specify these
	for i, t := range []string{"zeph-gcp-daily.json", "collection:exhaustive"} {
		if common.MatchTag(measurement.Tags, []string{t}, fAnalyzeTagsAnd) && e.Sub(s) > expectedDuration[i]*time.Hour {
			return DurationTooLong
		}
	}
	return DurationOK
}

func getMeasurements(args []string) ([]common.Measurement, error) {
	var measMdFile string
	if len(args) > 0 {
		measMdFile = args[0]
	} else {
		var err error
		measMdFile, err = meas.GetMeasMdFile(fAnalyzeAllUsers)
		if err != nil {
			return nil, err
		}
	}
	return common.GetMeasurementsSorted(measMdFile)
}

func validateFlags() {
	if len(fAnalyzeState) > 0 {
		if s, err := common.ValidateState(fAnalyzeState); err != nil {
			cliFatal(fmt.Sprintf("%v: %v", s, err))
		}
	}
}
