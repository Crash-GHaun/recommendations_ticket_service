// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.```
package bigqueryfunctions

import (
	"cloud.google.com/go/bigquery"
	"fmt"
	"encoding/json"
	"strings"
	"reflect"
	u "ticketservice/internal/utils"
)

type routingRow struct {
	Target string
	ProjectID string
	TicketSystemIdentifiers	[]string

}

var routingSchema = bigquery.Schema{
	{Name: "Target", Type: bigquery.StringFieldType, Required: true},
	{Name: "ProjectID", Type: bigquery.StringFieldType},
	{Name: "DeviceNamesOrLabels", Type: bigquery.StringFieldType, Repeated: true},
	{Name: "TicketSystemIdentifiers", Type: bigquery.StringFieldType, Repeated: true},
}

// JSONToArray takes a JSON string and returns an array of string{} values
func JSONToArray(jsonStr string) ([]string, error) {
    var array []string
    err := json.Unmarshal([]byte(jsonStr), &array)
    if err != nil {
        return nil, err
    }
    return array, nil
}

// ConvertArrayToString takes an array of strings and converts it to a single string
// with each element enclosed in double quotes and separated by commas.
func ConvertArrayToString(array []string) string {
    quotedArray := make([]string, len(array))
    for i, v := range array {
        quotedArray[i] = fmt.Sprintf("\"%s\"", v)
    }
    return strings.Join(quotedArray, ",")
}

var getTargetByProjectIDQuery = `Select * from %v.%v.%v 
									where ProjectID = "%v"
									limit 1`
var getTargetByDeviceAndProjectQuery = `
Select * from %v.%v.%v
where ProjectID = "%v"
and EXISTS(
	SELECT 1
	from UNNEST(DeviceNamesOrLabels) as d
	where d in (%v)
)
limit 1`

func GetRoutingRowsByProjectID(tableID string, project string, target_resource string, labels string)([]routingRow, error){
	u.log(1, "Labels: %v", labels)
	devicelabelarray, err := JSONToArray(labels)
	if err != nil {
		return nil, err
	}
	devicelabelarray = append(devicelabelarray, target_resource)
	query := fmt.Sprintf(
		getTargetByDeviceAndProjectQuery,
		projectID,
		datasetID,
		tableID,
		project,
		ConvertArrayToString(devicelabelarray),
	)
	t := reflect.TypeOf(routingRow{})
	results, err := QueryBigQueryToStruct(query, t)
	if err != nil {
		return nil, err
	}
	// If results don't exist, then default to project
	if len(results) < 1{
		query = fmt.Sprintf(getTargetByProjectIDQuery,projectID, datasetID, tableID, project)
		t = reflect.TypeOf(routingRow{})
		results, err = QueryBigQueryToStruct(query, t)
		if err != nil {
			return nil, err
		}
	}
	// Type assertion to convert results to []routingRow
	var rows []routingRow
	for _, row := range results {
		if r, ok := row.(routingRow); ok {
			rows = append(rows, r)
		} else {
			// Handle type assertion error
			return nil, fmt.Errorf("failed to assert type routingRow")
		}
	}

	return rows, nil
}

func CreateOrUpdateRoutingTable(tableID string) error {
	// Create the table if it does not already exist.
	if err := createTable(tableID, routingSchema); err != nil {
		return err
	}
	// Update the table schema if necessary.
	if err := updateTableSchema(tableID, routingSchema); err != nil {
		return err
	}
	// Return nil if the table was created or updated successfully.
	return nil
}