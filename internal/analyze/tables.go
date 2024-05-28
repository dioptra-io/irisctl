package analyze

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dioptra-io/irisctl/internal/clickhouse"
	"github.com/dioptra-io/irisctl/internal/common"
)

const (
	CHPROXYURL = "https://chproxy.iris.dioptra.io"
)

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

type MeasTable struct {
	Name    string `json:"name"`
	ModTime string `json:"metadata_modification_time"`
	Rows    int    `json:"total_rows"`
	Bytes   int    `json:"total_bytes"`
}

func getAllMeasTables() ([]MeasTable, error) {
	measTables := []MeasTable{}
	query := `SELECT
		    name,
		    metadata_modification_time,
		    total_rows,
		    total_bytes
		FROM
		    system.tables
		WHERE
		    name LIKE 'links__%' OR
		    name LIKE 'prefixes__%' OR
		    name LIKE 'probes__%' OR
		    name LIKE 'results__%'
		GROUP BY
		    name,
		    metadata_modification_time,
		    total_rows,
		    total_bytes
		ORDER BY
		    metadata_modification_time`
	filename, output, err := clickhouse.RunQueryString(query)
	if err != nil {
		fmt.Printf("%v\n", output)
		return measTables, err
	}
	return parseMeasTables(filename)
}

func getOneMeasTables(uuid string) ([]MeasTable, error) {
	measTables := []MeasTable{}
	query := `SELECT
		    name,
		    metadata_modification_time,
		    total_rows,
		    total_bytes
		FROM
		    system.tables
		WHERE
		    name LIKE '%` + strings.ReplaceAll(uuid, "-", "_") + "%'"
	filename, output, err := clickhouse.RunQueryString(query)
	if err != nil {
		fmt.Printf("%v\n", output)
		return measTables, err
	}
	return parseMeasTables(filename)
}

func parseMeasTables(filename string) ([]MeasTable, error) {
	contents, err := common.ReadCompressedFile(filename)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(contents, "\n")
	measTables := []MeasTable{}
	for _, line := range lines {
		if line == "" {
			continue
		}
		var t MeasTable
		if err := json.Unmarshal([]byte(line), &t); err != nil {
			return measTables, err
		}
		measTables = append(measTables, t)
	}
	return measTables, nil
}
