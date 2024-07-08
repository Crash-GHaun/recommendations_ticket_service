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

package main

import (
	"bytes"
	"fmt"
	"reflect"
	"sync"
	"text/template"

	b "ticketservice/internal/bigqueryfunctions"
	"ticketservice/internal/ticketinterfaces"
	u "ticketservice/internal/utils"
	q "ticketservice/internal/userspacequeries"
	"time"

)



func checkAndCreateNewTickets() error {
	var allowNullString string
	if c.AllowNullCost {
		allowNullString = "or impact_cost_unit is null"
	}
	query := fmt.Sprintf(ticketinterfaces.CheckQueryTpl, 
		fmt.Sprintf("%s.%s", c.BqDataset, c.BqRecommendationsTable),
		fmt.Sprintf("%s.%s", c.BqDataset, c.BqTicketTable),
		c.TicketCostThreshold,
		allowNullString,
		c.ExcludeSubTypes,
		c.TicketLimitPerCall,
	)
	u.LogPrint(1, "Querying for new Tickets")
	t := reflect.TypeOf(ticketinterfaces.RecommendationQueryResult{})
	results, err := b.QueryBigQueryToStruct(query, t)
	if err != nil {
		u.LogPrint(4,"Failed to query bigquery for new tickets")
		return err
	}
	var rowsToInsert []*ticketinterfaces.Ticket
	var rowsMutex sync.Mutex
	var wg sync.WaitGroup
	wg.Add(len(results))
	for _, r := range results{
		go func(r interface{}) error {
			defer wg.Done()
			row, ok := r.(ticketinterfaces.RecommendationQueryResult);
			if !ok {
				return fmt.Errorf("Failed to convert Query Schema into RecommendationQueryResults")
			}
			ticket := row.Ticket
			// Logic for if the ticket is already created
			if ticket.IssueKey != ""{
				u.LogPrint(3,"Already Exists: " + ticket.IssueKey)
				ticket.RecommenderID = row.RecommenderName
				ticket.SnoozeDate = time.Now().AddDate(0,0,7).Format(time.RFC3339)
				rowsMutex.Lock()
				rowsToInsert = append(rowsToInsert, ticket)
				rowsMutex.Unlock()
				return nil
			}
			u.LogPrint(1, "Retrieving Routing Information")
			routingRows, err := b.GetRoutingRowsByProjectID(
				c.BqRoutingTable,
				row.ProjectId,
				row.TargetResource,
				row.Labels)
			if err != nil {
				u.LogPrint(3,"Failed to get routing information: %v", err)
				return err
			}
			// Check if the length of routingRows is zero
			if len(routingRows) == 0 {
				u.LogPrint(3, "No routing rows found for the given project ID: %v", row.ProjectId)
				return fmt.Errorf("No routing rows found for the given project ID")
			}
			ticket.Status = "New"
			ticket.TargetResource = row.TargetResource
			ticket.RecommenderID = row.RecommenderName
			ticket.TargetContact = routingRows[0].Target
			ticket.Assignee = routingRows[0].TicketSystemIdentifiers
			u.LogPrint(1,"Creating new Ticket")
			ticketID, err := ticketService.CreateTicket(ticket, row)
			if err != nil {
				u.LogPrint(3, "Failed to create new ticket: %v", err)
				return err
			}
			ticket.IssueKey = ticketID
			rowsMutex.Lock()
			rowsToInsert = append(rowsToInsert, ticket)
			rowsMutex.Unlock()
			return nil
		}(r)
	}
	wg.Wait()
	if len(rowsToInsert) > 0 {
		err = b.AppendTicketsToTable(c.BqTicketTable, rowsToInsert)
		if err != nil {
			u.LogPrint(3,err)
			return err
		}
	}
	return err
}

func createUserSpaceTickets(query string, data map[string]interface{}) error {
	tmpl, err := template.New("query").Parse(query)
	if err != nil {
		return fmt.Errorf("error parsing query template: %v", err)
	}
	var queryBuffer bytes.Buffer
	if err := tmpl.Execute(&queryBuffer, data); err != nil {
		return fmt.Errorf("error executing query template: %v", err)
	}
	generatedQuery := queryBuffer.String()
	u.LogPrint(1, "Querying for new User Space Tickets")
	t := reflect.TypeOf(q.QueriesMap[query].ResponseStruct)
	results, err := b.QueryBigQueryToStruct(generatedQuery, t)

	u.LogPrint(1,"Results:", results)
	
	return nil
}