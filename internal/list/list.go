// Package list implements commands for listing of Iris measurement
// metadata (not in the Iris API).
package list

import (
	"errors"
	"fmt"
	"log"
	"time"

	//"github.com/dioptra-io/irisctl/internal/auth"
	"github.com/dioptra-io/irisctl/internal/common"
	"github.com/dioptra-io/irisctl/internal/meas"
	"github.com/spf13/cobra"
)

const (
	FirstMeasurementDate = "2020-01-01"
	LastMeasurementDate  = "2040-01-01" // distant future
)

var (
	// Command, its flags, subcommands, and their flags.
	//      list [--bq] [--all-users] [--before <yyyy-mm-dd>] [--after <yyyy-mm-dd>] [--state <state>]... [--tag <tag>]... [--tags-and] \
	//		[--agent <agent-hostname>...] [<meas-md-file>]
	//      list [--bq] --uuid <meas_uuid>...
	cmdName       = "list"
	subcmdNames   = []string{}
	fListAllUsers bool
	fListBQFormat bool
	fListBefore   common.CustomTime
	fListAfter    common.CustomTime
	fListState    []string
	fListTag      []string
	fListTagsAnd  bool
	fListAgents   []string
	fListUUID     bool

	// Errors.
	ErrInvalidTableName = errors.New("invalid table name")

	// Test code changes Fatal to Panic so a fatal error won't exit
	// the process and can be recovered.
	fatal    = log.Fatal
	cliFatal = common.CliFatal

	abbrState = map[string]string{
		"agent_failure": "E",
		"canceled":      "C",
		"finished":      "F",
		"ongoing":       "O",
	}
)

func init() {
	if err := fListAfter.Set(FirstMeasurementDate); err != nil {
		fatal(err)
	}
	if err := fListBefore.Set(LastMeasurementDate); err != nil {
		fatal(err)
	}
}

// ListCmd returns the command structure for list.
func ListCmd() *cobra.Command {
	listCmd := &cobra.Command{
		Use:       cmdName,
		Short:     "list measurements",
		Long:      "list measurements",
		ValidArgs: subcmdNames,
		Args:      listArgs,
		Run:       list,
	}
	listCmd.Flags().BoolVar(&fListAllUsers, "all-users", false, "match all measurements of all users (admin only)")
	listCmd.Flags().BoolVar(&fListBQFormat, "bq", false, "generate output suitable for inserting into BigQuery table")
	listCmd.Flags().Var(&fListBefore, "before", "match measurements before the specified date (exclusive)")
	listCmd.Flags().Var(&fListAfter, "after", "match measurements after the specified date (inclusive)")
	listCmd.Flags().StringArrayVarP(&fListState, "state", "s", []string{}, "repeatable: match measurements with the specified state (agent_failure, canceled, finished, ongoing)")
	listCmd.Flags().StringArrayVarP(&fListTag, "tag", "t", []string{}, "repeatable: match measurements with the specified tag (also see --tags-and)")
	listCmd.Flags().BoolVar(&fListTagsAnd, "tags-and", false, "match measurements that have all specified tags")
	listCmd.Flags().StringArrayVarP(&fListAgents, "agent", "a", []string{}, "repeatable: match measurements that ran on the specified agent")
	listCmd.Flags().BoolVarP(&fListUUID, "uuid", "", false, "list measurements with the specified UUIDs")
	listCmd.SetUsageFunc(common.Usage)
	listCmd.SetHelpFunc(common.Help)

	return listCmd
}

func listArgs(cmd *cobra.Command, args []string) error {
	if format, ok := common.IsUsage(args); ok {
		fmt.Printf(format, "<meas-md-file>", "optional: measurements metadata file")
		return nil
	}
	if !fListUUID && len(args) > 1 {
		cliFatal("list takes at most one argument: <meas-md-file>")
	}
	if fListUUID && len(args) < 1 {
		cliFatal("list --uuid requires at least one argument: <meas-uuid>...")
	}
	validateFlags()
	if fListAllUsers && len(args) > 0 {
		fmt.Printf("WARNING: ignoring --all-users because a measurement metadata file is specidfied\n")
		fListAllUsers = false
	}
	return nil
}

// TODO: This function is pretty ugly and needs to be refactored.
func list(cmd *cobra.Command, args []string) {
	if fListUUID {
		for _, arg := range args {
			measurement, err := meas.GetMeasurementAllDetails(arg)
			if err != nil {
				fatal(err)
			}
			if fListBQFormat {
				printMeasDetailsBQ(measurement)
			} else {
				printMeasDetails(measurement)
			}
		}
	} else {
		measurements, err := getMeasurements(args)
		if err != nil {
			fatal(err)
		}
		for _, measurement := range measurements {
			if measSkip(measurement) {
				continue
			}
			if fListBQFormat {
				measurement, err = meas.GetMeasurementAllDetails(measurement.UUID)
				if err != nil {
					fatal(err)
				}
				printMeasDetailsBQ(measurement)
			} else {
				printMeasDetails(measurement)
			}
		}
	}
}

func getMeasurements(args []string) ([]common.Measurement, error) {
	var measMdFile string
	if len(args) > 0 {
		measMdFile = args[0]
	} else {
		var err error
		measMdFile, err = meas.GetMeasMdFile(fListAllUsers)
		if err != nil {
			return nil, err
		}
	}
	return common.GetMeasurementsSorted(measMdFile)
}

func measSkip(measurement common.Measurement) bool {
	if len(fListTag) > 0 && !common.MatchTag(measurement.Tags, fListTag, fListTagsAnd) {
		return true
	}
	if len(fListState) > 0 && !common.MatchState(measurement.State, fListState) {
		return true
	}
	if !measurement.CreationTime.After(fListAfter.Time) ||
		!measurement.CreationTime.Before(fListBefore.Time) {
		return true
	}
	return false
}

func printMeasDetails(measurement common.Measurement) {
	fmt.Printf("%s", measurement.UUID)
	if common.RootFlagBool("brief") {
		fmt.Println()
		return
	}
	c := time.Time(measurement.CreationTime.Time)
	s := time.Time(measurement.StartTime.Time)
	e := time.Time(measurement.EndTime.Time)
	a, ok := abbrState[measurement.State]
	if !ok {
		panic("internal error: invalid measurement state")
	}
	fmt.Printf(" %2d %s  ", len(measurement.Agents), a)
	fmt.Printf("%s   ", c.Format("06-01-02.15:04:05"))
	fmt.Printf("%s %3.fs  ", s.Format("06-01-02.15:04:05"), s.Sub(c).Seconds())
	fmt.Printf("%s %10s  ", e.Format("06-01-02.15:04:05"), e.Sub(s).Round(time.Second))
	fmt.Printf("%q", measurement.Tags)
	fmt.Println()
}

func printMeasDetailsBQ(measurement common.Measurement) {
	fmt.Printf("%s,", measurement.UUID) // uuid

	s := time.Time(measurement.StartTime.Time)
	fmt.Printf("%s,", s.Format("2006-01-02 15:04:05")) // start_time
	e := time.Time(measurement.EndTime.Time)
	fmt.Printf("%s,", e.Format("2006-01-02 15:04:05")) // end_time

	fmt.Printf("%s,", measurement.State) // state

	fmt.Printf("%d,", len(measurement.Agents)) // agents_num
	agents_finished := 0
	for i := 0; i < len(measurement.Agents); i++ {
		if measurement.Agents[i].State == "finished" {
			agents_finished++
		}
	}
	fmt.Printf("%d\n", agents_finished) // agents_finished
}

func validateFlags() {
	if len(fListState) > 0 {
		if s, err := common.ValidateState(fListState); err != nil {
			cliFatal(fmt.Sprintf("%v: %v", s, err))
		}
	}
}
