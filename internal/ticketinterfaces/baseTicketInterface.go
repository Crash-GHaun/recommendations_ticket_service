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

package ticketinterfaces

import (
	"plugin"
	"github.com/labstack/echo/v4"
	
	u "ticketservice/internal/utils"
)

// Your plugin needs to have the method CreateService that returns your BaseTicketService interface implementation

// TicketService is an interface for managing tickets.
type BaseTicketService interface {
	Init() error
	// I might want to update this to not return anything, except err. Because we are modifying the 
	// original variable anyways. 
	CreateTicket(ticket *Ticket, row RecommendationQueryResult) (string, error)
	UpdateTicket(ticket *Ticket, row RecommendationQueryResult) error
	CloseTicket(issueKey string) error
	GetTicket(issueKey string) (Ticket, error)
	HandleWebhookAction(echo.Context) error
}

func InitTicketService(implName string) (BaseTicketService, error) {

	// Load the plugin based on the name
	pluginPath := "./plugins/" + implName + ".so"
	p, err := plugin.Open(pluginPath)
	if err != nil {
		u.LogPrint(1, "Plugin name: %v", implName)
		u.LogPrint(4, "Failed to open plugin: %v", err)
	}

	// Look up the "NewTicketService" symbol in the plugin
	newTicketServiceSymbol, err := p.Lookup("CreateService")
	if err != nil {
		u.LogPrint(4, "Failed to lookup symbol: %v", err)
	}

	// Create an instance of the ticket service implementation
	implValue := newTicketServiceSymbol.(func() BaseTicketService)()

	// Initialize the ticket service implementation
	if err := implValue.Init(); err != nil {
		return nil, err
	}

	return implValue, nil
}

// %[1] is the recommender export table
// %[2] is the ticket table
// %[3] is the Cost Threshold
// %[4] is an additional string added to allow null values
// %[5] is a subtype filter
// %[6] is the limit of rows
// The Format timestamp works here, but doesn't work in ticketTableFunctions? 
// If it stops working here try changing to '%%Y-%%m-%%d %%H:%%M:%%S'
var CheckQueryTpl = `SELECT
  IFNULL(f.project_name, "") as ProjectName,
  IFNULL(f.project_id, "") as ProjectID,
  f.recommender_name as RecommenderName,
  f.location as Location,
  f.recommender_subtype as RecommenderSubtype,
  f.impact_cost_unit as ImpactCostUnit,
  f.impact_currency_code as ImpactCurrencyCode,
  f.description as Description,
  TargetResource,
  STRUCT(
    IFNULL(t.IssueKey, "") AS IssueKey,
    IFNULL(t.TargetContact, "") AS TargetContact,
    FORMAT_TIMESTAMP('%%FT%%T%%z', IFNULL(t.CreationDate, TIMESTAMP '1970-01-01T00:00:00Z')) AS CreationDate,
    IFNULL(t.Status, "") AS Status,
    IFNULL(t.TargetResource, "") AS TargetResource,
    IFNULL(t.RecommenderID, "") AS RecommenderID,
    FORMAT_TIMESTAMP('%%FT%%T%%z', IFNULL(t.LastUpdateDate, TIMESTAMP '1970-01-01T00:00:00Z')) AS LastUpdateDate,
    FORMAT_TIMESTAMP('%%FT%%T%%z', IFNULL(t.LastPingDate, TIMESTAMP '1970-01-01T00:00:00Z')) AS LastPingDate,
    FORMAT_TIMESTAMP('%%FT%%T%%z', IFNULL(t.SnoozeDate, TIMESTAMP '1970-01-01T00:00:00Z')) AS SnoozeDate,
    IFNULL(t.Subject, "") AS Subject,
    t.Assignee
  ) AS Ticket
FROM %[1]s AS f
CROSS JOIN UNNEST(target_resources) AS TargetResource
LEFT JOIN (
	SELECT *,
		   ROW_NUMBER() OVER (PARTITION BY IssueKey ORDER BY LastUpdateDate DESC) as rn
	FROM %[2]s
  ) AS t ON TargetResource = t.TargetResource AND t.rn = 1
WHERE (t.IssueKey IS NULL OR CURRENT_TIMESTAMP() >= SnoozeDate)
  AND (impact_cost_unit >= %[3]d %[4]s)
  AND recommender_subtype NOT IN (%[5]s)
LIMIT %[6]d`