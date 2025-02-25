// Package common implements common code used by other packages.
package common

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	AuthAPISuffix         = "/auth"
	UsersAPISuffix        = "/users"
	AgentsAPISuffix       = "/agents"
	TargetsAPISuffix      = "/targets"
	MeasurementsAPISuffix = "/measurements"
	StatusAPISuffix       = "/status"
	MaintenanceAPISuffix  = "/maintenance"

	UserID          = "user ID"
	MeasurementUUID = "measurement UUID"

	GCPProject = "mlab-edgenet"

	UserFile = `
{
  "email": "user@example.com",
  "password": "string",
  "is_active": true,
  "is_superuser": false,
  "is_verified": false,
  "firstname": "string",
  "lastname": "string",
  "probing_enabled": false,
  "probing_limit": 1,
  "allow_tag_reserved": false,
  "allow_tag_public": true
}`

	TargetListFile = `
Each line of the target-list file must have the following format:
target,protocol,min_ttl,max_ttl,n_initial_flows
where the target is an IPv4/IPv6 prefix or IPv4/IPv6 address
and the prococol is icmp, icmp6 or udp.`

	ProbeListFile = `
Each line of the probe-list file must have the following format:
dst_addr,src_port,dst_port,ttl,protocol
where target is an IPv4/IPv6 prefix or IPv4/IPv6 address
and the prococol is icmp, icmp6 or udp.`

	MeasurementFile = `
{
  "tool": "diamond-miner",
  "agents": [
    {
      "tag": "all",
      "target_file": "prefixes.csv"
    }
  ],
  "tags": [
    "test"
  ]
}`

	// Width of usage column.
	UsageWidth     = 40
	UsageSignature = "usagesignature"
)

type Users struct {
	Count    int     `json:"count"`
	Next     *string `json:"next"`
	Previous *string `json:"previous"`
	Results  []User  `json:"results"`
}

type User struct {
	UUID              string     `json:"id"`
	Email             string     `json:"email"`
	IsActive          bool       `json:"is_active"`
	IsSuperuser       bool       `json:"is_superuser"`
	IsVerified        bool       `json:"is_verified"`
	FirstName         string     `json:"firstname"`
	LastName          string     `json:"lastname"`
	ProbingEnabled    bool       `json:"probing_enabled"`
	ProbingLimit      int32      `json:"probing_limit"`
	AllowTagReserveed bool       `json:"allow_tag_reserved"`
	AllowTagPublic    bool       `json:"allow_tag_public"`
	CreationTime      CustomTime `json:"creation_time"`
}

// CustomTime defines a custom time used by Iris.
type CustomTime struct {
	time.Time
}

// Agent defines an Iris agent.
type Agent struct {
	ToolParameters    ToolParameters  `json:"tool_parameters"`
	AgentParameters   AgentParameters `json:"agent_parameters"`
	BatchSize         interface{}     `json:"batch_size"`
	ProbingRate       interface{}     `json:"probing_rate"`
	TargetFile        string          `json:"target_file"`
	AgentUUID         string          `json:"agent_uuid"`
	ProbingStatistics map[string]struct {
		Round struct {
			Limit  int `json:"limit"`
			Number int `json:"number"`
			Offset int `json:"offset"`
		} `json:"round"`
		EndTime                string `json:"end_time"`
		StartTime              string `json:"start_time"`
		ProbesRead             int    `json:"probes_read"`
		PacketsSent            int    `json:"packets_sent"`
		PcapDropped            int    `json:"pcap_dropped"`
		PcapReceived           int    `json:"pcap_received"`
		PacketsFailed          int    `json:"packets_failed"`
		FilteredLow_ttl        int    `json:"filtered_low_tll"`
		PacketsReceived        int    `json:"packets_received"`
		FilteredHigh_ttl       int    `json:"filtered_high_ttl"`
		FilteredPrefix_excl    int    `json:"filtered_prefix_excl"`
		PcapInterfaceDropped   int    `json:"pcap_interface_dropped"`
		FilteredPrefixNotIncl  int    `json:"filtered_prefix_not_incl"`
		PacketsReceivedInvalid int    `json:"packets_received_invalid"`
	} `json:"probing_statistics"`
	State string `json:"state"`
}

// AgentOld defines an Iris agent.
type AgentOld struct {
	ToolParameters    ToolParameters  `json:"tool_parameters"`
	AgentParameters   AgentParameters `json:"agent_parameters"`
	BatchSize         interface{}     `json:"batch_size"`
	ProbingRate       interface{}     `json:"probing_rate"`
	TargetFile        string          `json:"target_file"`
	AgentUUID         string          `json:"agent_uuid"`
	ProbingStatistics map[string]struct {
		Round                  string `json:"round"`
		EndTime                string `json:"end_time"`
		StartTime              string `json:"start_time"`
		ProbesRead             int    `json:"probes_read"`
		PacketsSent            int    `json:"packets_sent"`
		PcapDropped            int    `json:"pcap_dropped"`
		PcapReceived           int    `json:"pcap_received"`
		PacketsFailed          int    `json:"packets_failed"`
		FilteredLow_ttl        int    `json:"filtered_low_tll"`
		PacketsReceived        int    `json:"packets_received"`
		FilteredHigh_ttl       int    `json:"filtered_high_ttl"`
		FilteredPrefix_excl    int    `json:"filtered_prefix_excl"`
		PcapInterfaceDropped   int    `json:"pcap_interface_dropped"`
		FilteredPrefixNotIncl  int    `json:"filtered_prefix_not_incl"`
		PacketsReceivedInvalid int    `json:"packets_received_invalid"`
	} `json:"probing_statistics"`
	State string `json:"state"`
}

// Measurement defines an Iris measurement.
type Measurement struct {
	Tool         string     `json:"tool"`
	Tags         []string   `json:"tags"`
	UUID         string     `json:"uuid"`
	UserID       string     `json:"user_id"`
	CreationTime CustomTime `json:"creation_time"`
	StartTime    CustomTime `json:"start_time"`
	EndTime      CustomTime `json:"end_time"`
	State        string     `json:"state"`
	Agents       []Agent    `json:"agents"`
}

// MeasurementOld defines an old Iris measurement.
type MeasurementOld struct {
	Tool         string     `json:"tool"`
	Tags         []string   `json:"tags"`
	UUID         string     `json:"uuid"`
	UserID       string     `json:"user_id"`
	CreationTime CustomTime `json:"creation_time"`
	StartTime    CustomTime `json:"start_time"`
	EndTime      CustomTime `json:"end_time"`
	State        string     `json:"state"`
	Agents       []AgentOld `json:"agents"`
}

// MeasurementBatch defines a batch of measurements returned by Iris API.
type MeasurementBatch struct {
	Count        int           `json:"count"`
	Next         *string       `json:"next"`
	Previous     *string       `json:"previous"`
	Measurements []Measurement `json:"results"`
}

type AgentsData struct {
	Count    int            `json:"count"`
	Next     string         `json:"next"`
	Previous string         `json:"previous"`
	Results  []AgentsResult `json:"results"`
}

type AgentsResult struct {
	UUID       string          `json:"uuid"`
	State      string          `json:"state"`
	Parameters AgentParameters `json:"parameters"`
}

type AgentParameters struct {
	Version             string   `json:"version"`
	Hostname            string   `json:"hostname"`
	InternalIPv4Address string   `json:"internal_ipv4_address"`
	InternalIPv6Address string   `json:"internal_ipv6_address"`
	ExternalIPv4Address string   `json:"external_ipv4_address"`
	ExternalIPv6Address string   `json:"external_ipv6_address"`
	CPUs                int      `json:"cpus"`
	Disk                float64  `json:"disk"`
	Memory              float64  `json:"memory"`
	MinTTL              int      `json:"min_ttl"`
	MaxProbingRate      int      `json:"max_probing_rate"`
	Tags                []string `json:"tags"`
}

type ToolParameters struct {
	InitialSourcePort  int     `json:"initial_source_port"`
	DestinationPort    int     `json:"destination_port"`
	MaxRound           int     `json:"max_round"`
	FailureProbability float64 `json:"failure_probability"`
	FlowMapper         string  `json:"flow_mapper"`
	FlowMapperKwargs   struct {
		Seed int `json:"seed"`
	} `json:"flow_mapper_kwargs"`
	PrefixLenV4  int `json:"prefix_len_v4"`
	PrefixLenV6  int `json:"prefix_len_v6"`
	GlobalMinTTL int `json:"global_min_ttl"`
	GlobalMaxTTL int `json:"global_max_ttl"`
}

type ClickHouse struct {
	BaseURL  string `json:"base_url"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type S3 struct {
	AWKAccessKeyId     string `json:"aws_access_key_id"`
	AWSSecretAccessKey string `json:"aws_secret_access_key"`
	AWSSessionToekn    string `json:"aws_session_token"`
	EndPointURL        string `json:"endpoint_url"`
}

type MeServices struct {
	ClickHouse        ClickHouse `json:"clickhouse"`
	ClickHouseExpTime time.Time  `json:"clickhouse_expiration_time"`
	S3                S3         `json:"s3"`
	S3ExpTime         time.Time  `json:"s3_expiration_time"`
}

var (
	GCPHOSTNAMES = []string{
		"iris-asia-east1",
		"iris-asia-northeast1",
		"iris-asia-south1",
		"iris-asia-southeast1",
		"iris-europe-north1",
		"iris-europe-west6",
		"iris-me-central1",
		"iris-southamerica-east1",
		"iris-us-east4",
		"iris-us-west4",
	}

	calledFromHelp bool

	// Errors.
	ErrNoSubCmd       = errors.New("missing subcommand")
	ErrHomeEnv        = errors.New("HOME environment variable is not set")
	ErrNotRegularFile = errors.New("not a regular file")
	ErrZeroLength     = errors.New("zero length file")
	ErrInvalidLine    = errors.New("invalid line")
	ErrInvalidState   = errors.New("invalid state")
	ErrInvalidUUID    = errors.New("invalid UUID")

	// Test code changes Fatal to Panic so a fatal error won't exit
	// the process and can be recovered.
	fatal = log.Fatal
)

// Set implements the pflag.Value interface Set method.
func (c *CustomTime) Set(value string) error {
	parsedTime, err := time.Parse("2006-01-02T15:04:05.999999", value)
	if err != nil {
		return err
	}
	c.Time = parsedTime
	return nil
}

// Type implements the pflag.Value interface Type method.
func (c *CustomTime) Type() string {
	return "CustomTime"
}

// UnmarshalJSON implements the unmarshal method.
func (c *CustomTime) UnmarshalJSON(b []byte) error {
	s := string(b)
	if s == "null" || s == "" {
		c.Time = time.Time{}
		return nil
	}
	date, err := time.Parse(`"2006-01-02T15:04:05.999999"`, string(b))
	if err != nil {
		return err
	}
	c.Time = date
	return nil
}

// Less returns true if the measurement time of the measurement
// argument is earlier.
func (m Measurement) Less(t CustomTime) bool {
	return time.Time(m.CreationTime.Time).Before(t.Time)
}

func RootFlagBool(flag string) bool {
	return viper.GetBool(flag)
}

func RootFlagString(flag string) string {
	return viper.GetString(flag)
}

func APIEndpoint(endpoint string) string {
	return RootFlagString("iris-api-url") + endpoint
}

func CliFatal(args ...interface{}) {
	log.SetFlags(0)
	log.SetPrefix("")
	log.Fatal(args...)
}

func Verbose(s string, args ...interface{}) {
	if RootFlagBool("verbose") {
		fmt.Printf(s, args...)
	}
}

func Usage(cmd *cobra.Command) error {
	if !calledFromHelp {
		os.Exit(1)
	}
	fmt.Printf("%-*s%s\n", UsageWidth, cmd.Use, cmd.Long)
	printFlagsArgs(cmd, cmd)
	for _, c := range cmd.Commands() {
		name := c.Name()
		if name == "completion" || name == "help" {
			continue
		}
		blanks, width := tabulate(cmd, c, false, 1)
		fmt.Printf("%s%-*s%s\n", blanks, width, name, c.Long)
		printFlagsArgs(cmd, c)
		for _, sc := range c.Commands() {
			blanks, width := tabulate(cmd, sc, false, 2)
			fmt.Printf("%s%-*s%s\n", blanks, width, sc.Name(), sc.Long)
			printFlagsArgs(cmd, sc)
		}
	}
	return nil
}

func IsUsage(args []string) (string, bool) {
	if len(args) == 2 && args[0] == UsageSignature {
		return args[1], true
	}
	return "", false
}

func Help(cmd *cobra.Command, args []string) {
	if cmd.Use == "irisctl" && len(args) > 0 && args[0] != "-h" && args[0] != "--help" {
		CliFatal("unknown command: ", args[0])
	}
	calledFromHelp = true
	if err := cmd.Usage(); err != nil {
		fatal(err)
	}
}

func ValidateFormat(args []string, what string) error {
	var re string
	switch what {
	case UserID:
		fallthrough
	case MeasurementUUID:
		re = "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"
	default:
		fatal(what)
	}

	for _, arg := range args {
		match, err := regexp.MatchString(re, arg)
		if err != nil {
			fatal(err)
		}
		if !match {
			// CliFatal("invalid ", what, " format: ", arg)
			return fmt.Errorf("%v: %v", arg, ErrInvalidUUID)
		}
	}
	return nil
}

func Contains(ss []string, s string) bool {
	for _, t := range ss {
		if t == s {
			return true
		}
	}
	return false
}

func Curl(accessToken string, basicToken bool, method, url string, args ...string) ([]byte, error) {
	var curlArgs []string
	curlArgs = append(curlArgs, "-s", "-X", method, "-H", "User-Agent: irisctl", "-H", "Accept: application/json")
	if accessToken != "" {
		if basicToken {
			encodedToken := base64.StdEncoding.EncodeToString([]byte(accessToken))
			curlArgs = append(curlArgs, "-H", fmt.Sprintf("Authorization: Basic %s", encodedToken))
		} else {
			curlArgs = append(curlArgs, "-H", fmt.Sprintf("Authorization: Bearer %s", accessToken))
		}
	}
	curlArgs = append(curlArgs, args...)
	curlArgs = append(curlArgs, url)
	if RootFlagBool("curl") || RootFlagBool("verbose") {
		fmt.Printf("curl ")
		for _, a := range curlArgs {
			fmt.Printf("%q ", a)
		}
		fmt.Println()
		if RootFlagBool("curl") {
			return nil, nil
		}
	}
	cmd := exec.Command("curl", curlArgs...)
	return cmd.CombinedOutput()
}

func CheckFile(desc, path string) (os.FileInfo, error) {
	Verbose("checking %s file %s\n", desc, path)
	fi, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, ErrNotRegularFile
	}
	return fi, nil
}

func WriteResults(file string, data []byte) (string, error) {
	tmpFile, err := os.CreateTemp("/tmp", file+"-")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()
	if viper.GetBool("no-delete") {
		fmt.Printf("saving results in %s\n", tmpFile.Name())
	}
	if _, err := tmpFile.Write(data); err != nil {
		return tmpFile.Name(), err
	}
	return tmpFile.Name(), nil
}

func WriteResultsAppend(file string, data []byte) (string, error) {
	tmpFile, err := os.CreateTemp("/tmp", file+"-")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()
	if _, err := tmpFile.Write(data); err != nil {
		return tmpFile.Name(), err
	}
	return tmpFile.Name(), nil
}

func SaveOrPrint(jsonData []byte, prefix string) error {
	if RootFlagBool("stdout") {
		jqOutput, err := JqBytes(jsonData, []string{RootFlagString("jq-filter")})
		if err != nil {
			return err
		}
		fmt.Println(string(jqOutput))
	} else {
		f, err := os.CreateTemp("/tmp", prefix)
		if err != nil {
			return err
		}
		defer f.Close()
		fmt.Fprintf(os.Stderr, "saving in %s\n", f.Name())
		if _, err := f.Write(jsonData); err != nil {
			return err
		}
	}
	return nil
}

func JqFile(file string, filter []string) ([]byte, error) {
	args := append(filter, file)
	cmd := exec.Command("jq", args...)
	return runCmd(cmd)
}

func JqBytes(jsonData []byte, filter []string) ([]byte, error) {
	cmd := exec.Command("jq", filter...)
	cmd.Stdin = bytes.NewBuffer(jsonData)
	return runCmd(cmd)
}

func GcloudSSH(hostname, remoteCmd string) ([]string, error) {
	zone := strings.TrimPrefix(hostname, "iris-") + "-a"
	cmd := exec.Command("gcloud", "compute", "ssh", "--zone", zone, hostname, "--project", GCPProject, "--command", remoteCmd, "--", "-t", "-t")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%v\n%v\n", string(output), err)
	}
	var results []string
	results = append(results, fmt.Sprintf("%s\n", hostname))
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line != "" {
			results = append(results, fmt.Sprintf("%s\n", line))
		}
	}
	return results, nil
}

func GetMeasurementsSorted(measMdFile string) ([]Measurement, error) {
	Verbose("parsing measurements metadata file %s\n", measMdFile)
	file, err := os.Open(measMdFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	var allMeasurements []Measurement
	for {
		var batch MeasurementBatch
		if err := decoder.Decode(&batch); err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, err
		}
		allMeasurements = append(allMeasurements, batch.Measurements...)
	}
	sort.Slice(allMeasurements, func(i, j int) bool {
		t := allMeasurements[j].CreationTime
		return allMeasurements[i].Less(t)
	})
	return allMeasurements, nil
}

func ValidateState(states []string) (string, error) {
	for _, state := range states {
		switch state {
		case "finished":
		case "ongoing":
		case "canceled":
		case "agent_failure":
		default:
			return state, ErrInvalidState
		}
	}
	return "", nil
}

func MatchState(mState string, states []string) bool {
	for _, state := range states {
		if state == mState {
			return true
		}
	}
	return false
}

func MatchTag(mTags, tags []string, tagsAnd bool) bool {
	var found bool
	for _, tag := range tags {
		found = false
		for _, measTag := range mTags {
			if strings.Contains(strings.ToLower(measTag), tag) {
				found = true
				break
			}
		}
		if found && !tagsAnd {
			return true
		}
		if !found && tagsAnd {
			return false
		}
	}
	return found
}

func HumanReadable(n int) string {
	switch {
	case n >= 1000000000:
		return fmt.Sprintf("%.1fB", float64(n)/1000000000.0)
	case n >= 1000000:
		return fmt.Sprintf("%.1fM", float64(n)/1000000.0)
	case n >= 1000:
		return fmt.Sprintf("%.1fK", float64(n)/1000.0)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func ParseGCPHostnames(jsonData []byte) ([]string, error) {
	var data AgentsData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, err
	}
	gcpHostnames := []string{}
	re := regexp.MustCompile("^[a-z](?:[-a-z0-9]{0,61}[a-z0-9])?$")
	for _, result := range data.Results {
		if re.Match([]byte(result.Parameters.Hostname)) {
			gcpHostnames = append(gcpHostnames, result.Parameters.Hostname)
		}
	}
	return gcpHostnames, nil
}

func ReadCompressedFile(filename string) (string, error) {
	fileInfo, err := os.Stat(filename)
	if err != nil {
		return "", err
	}
	if fileInfo.Size() == 0 {
		return "", fmt.Errorf("%v: %w", filename, ErrZeroLength)
	}
	gziped, err := isGzipFile(filename)
	if err != nil {
		return "", err
	}
	var contents []byte
	if gziped {
		cmd := exec.Command("gunzip", "-c", filename)
		if contents, err = cmd.CombinedOutput(); err != nil {
			fmt.Printf("%v\n", string(contents))
			return "", err
		}
	} else {
		contents, err = os.ReadFile(filename)
		if err != nil {
			return "", err
		}
	}
	return string(contents), nil
}

func isGzipFile(filename string) (bool, error) {
	file, err := os.Open(filename)
	if err != nil {
		return false, err
	}
	defer file.Close()

	magic := make([]byte, 2)
	_, err = file.Read(magic)
	if err != nil {
		return false, err
	}
	return magic[0] == 0x1f && magic[1] == 0x8b, nil
}

func printFlagsArgs(parentCmd, cmd *cobra.Command) {
	var flags *pflag.FlagSet
	if !cmd.HasParent() {
		flags = cmd.Flags()
	} else {
		flags = cmd.LocalNonPersistentFlags()
	}
	if flags == nil {
		return
	}
	// Print flags of this command/subcommand.
	flags.VisitAll(func(flg *pflag.Flag) {
		if flg.Shorthand == "h" || flg.Name == "help" {
			return
		}
		var f string
		if flg.Shorthand != "" {
			f = fmt.Sprintf("-%s --%s", flg.Shorthand, flg.Name)
		} else {
			f = fmt.Sprintf("--%s", flg.Name)
		}
		blanks, width := tabulate(parentCmd, cmd, true, 3)
		fmt.Printf("%s%-*s%s\n", blanks, width, f, flg.Usage)
	})
	// If this command/subcommand takes arguments, print out their usage.
	if cmd.Args != nil {
		blanks, width := tabulate(parentCmd, cmd, true, 4)
		format := fmt.Sprintf("%s%%-%ds%%s\n", blanks, width)
		if err := cmd.Args(cmd, []string{UsageSignature, format}); err != nil {
			fatal(err)
		}
	}
}

func tabulate(parentCmd, cmd *cobra.Command, isFlagsArgs bool, id int) (string, int) {
	var blanks string
	if isFlagsArgs {
		blanks = "    "
	}
	if parentCmd.Use != cmd.Use {
		for p := cmd.Parent(); p != nil; p = p.Parent() {
			blanks = blanks + "    "
			if p == parentCmd {
				break
			}
		}
	}
	width := UsageWidth - len(blanks)
	if width < 0 {
		width = 0
	}
	return blanks, width
}

func runCmd(cmd *exec.Cmd) ([]byte, error) {
	Verbose("%v\n", cmd)
	return cmd.CombinedOutput()
}
