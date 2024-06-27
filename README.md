# Ticket Service

This is a Go based microservice designed to handle ticketing logic. It includes features like creating, closing tickets and managing webhooks for ticket operations. The service also interacts with Google's BigQuery for data handling. 

## Getting Started

These instructions will get you a copy of the project up and running on your local machine for development and testing purposes.

### Prerequisites

Ensure that you have the latest version of Go installed on your machine. The code is tested with Go version 1.19. However, it should work with newer versions as well.

**Note:**  The current configuration here assumes you are using Application Default Credentials for BQ Access.

You also need to have Recommender API exports and Asset Inventory exports configured to send to BigQuery. [Please see our Workflow for automating this process](org-level-recommendations-hub/workflows)

### Installing

- Clone the repository to your local machine
```
git clone https://github.com/Crash-GHaun/active-assist.git
```

- Navigate into the cloned repository
```
cd org-level-recommendations-hub/ticketservice
```

- Download Dependencies
```
go mod download
```

-- Compile the plugins
```
go run compilePlugins.go
```

- To start the service locally, run the following command
```
go run main.go ticketLogic.go
```

If you modify the protos you will need to run:
```
protoc --go_out=../ --go_opt=paths=source_relative *.proto
```

## Configuration

Configuration is handled through environment variables. The list of required and optional environment variables are:

- BQ_DATASET **(required)**
  - BigQuery Dataset that contains exported recommendations
- BQ_PROJECT **(required)**
  - BigQuery Project your dataset is in.
- BQ_RECOMMENDATIONS_TABLE (optional, defaults to "flattened_recommendations")
  - Table/View name of your [Flattened Recommendations](org-level-recommendations-hub/flatten-table-bigquery.sql)
- BQ_TICKET_TABLE (optional, defaults to "recommender_ticket_table")
  - The name of the table you want to use for storing ticket data
- BQ_ROUTING_TABLE (optional, defaults to "recommender_routing_table")
  - The name of the table that stores project to target and system identifiers. See [Routing Table](#routing-table) for more information.
- TICKET_SERVICE_IMPL (optional, defaults to "slackTicket")
  - The Ticket Service Implementation you want to use. I.E (slackTicket). This should match the name of the plugin without the .so extension.
- TICKET_COST_THRESHOLD (optional, defaults to 100)
  - Limits the creation of tickets to a certain monetary threshold. 
- TICKET_LIMIT (optional, defaults to 5)
  - You can limit the amount of tickets created per call to reduce spam
- ALLOW_NULL_COST (optional, defaults to "false")
  - This allows you to create tickets for recommendations that **do not** have costs associated with them.
- EXCLUDE_SUB_TYPES (optional, defaults to ' ')
  - A Comma seperated list that allows you to filter the types of recommendations that recieve tickets.

Please note that the environment variables needs to be set before starting the service.

## Template-Based Messaging

This service now uses Go templates for generating ticket messages and titles. Templates are loaded from files and filled in with data from the `RecommendationQueryResult` and `Ticket` structs.

### Template Files

Two template files are used:

1. `messageTemplate.txt` - For the body of the ticket message.
2. `ticketTitleTemplate.txt` - For the ticket title.

These templates use placeholders like `{{.Row.Project_name}}` to insert specific fields from the structs. The templates are loaded and parsed during the service initialization and are stored in memory for efficient reuse.

### Usage

To use these templates, simply execute them with the corresponding data structures:

- `messageTemplate.Execute(&buffer, data)`
- `ticketTitleTemplate.Execute(&buffer, data)`

### Struct Location

The structs used in these templates (`RecommendationQueryResult` and `Ticket`) are defined in `internal/ticketInterfaces/baseTicketInterface.go`.

### Modifying Templates

To modify the behavior of message or title generation, edit the respective template files. This approach allows for easy changes without modifying the core service code.

## Endpoints

- `GET /CreateTickets`: Checks for new tickets, and Updates stale tickets.
- `POST /tickets`: Creates a new ticket.
- `PUT /tickets/:issueKey/close`: Closes an existing ticket.
- `POST /webhooks`: Handles webhook actions based on your ticket service.

## Deployment

This service is deployed using Google Cloud Build and Docker. 

### Docker

A Dockerfile is included in the repository. The Dockerfile uses a two-stage build process. In the first stage, it compiles the Go code to create an executable.It also compiles the plugins included in this repo.

In the second stage, it copies the compiled binary into a new Docker image.


### Google Cloud Build

Cloud Build is configured to build the Dockerfile, push the image to Google Container Registry, and then deploy the image to Google Cloud Run with some environment variables set. To use this build file you must set the following substitutions.

- _SERVICE
  - Name of the Cloud Run Service
- _REGION
- _BQ_DATASET
- _LOG_LEVEL
- _SLACK_CHANNEL_AS_TICKET
- _TICKET_COST_THRESHOLD

If you are using the Slack integration you will need the following secrets configured.

- SLACK_SIGNING_SECRET
- SLACK_API_TOKEN

## Routing Table

The Ticket Service relies on a BigQuery table for routing tickets to the appropriate person or team. This table contains the following schema:

```go
bigquery.Schema{
    {Name: "Target", Type: bigquery.StringFieldType, Required: true},
    {Name: "ProjectID", Type: bigquery.StringFieldType},
    {Name: "TicketSystemIdentifiers", Type: bigquery.StringFieldType, Repeated: true},
}
```

### Ticket Routing

As of the current version, all routing of recommendations is done starting at the `ProjectID`. Each recommendation gets mapped to a `ProjectID`, which then provides the necessary routing information for ticket creation. If further routing needs to be accomplished it can be accomplished by adding Device/VM names or labels to the field 'DeviceNamesOrLabels'. 

### Target Field

The `Target` field is determined by the desired location or component where the ticket will be created. This is based on the specific ticket implementation in use.

For example, if you are using Slack (with the `SLACK_CHANNEL_AS_TICKET` environment variable set to `false`), the `Target` would be the Slack channel name where a thread should be initiated.

### DeviceNamesOrLabels Field

On testing it became apparnt that routing soly on projectID might not be enough for large enterprises. In order to further route tickets you can either route by device name (VM name for now) or by Labels associated with the machine. To held decrease BQ Rows this field is repeated, so it can accept multiple devices or labels. 

### TicketSystemIdentifiers Field

The `TicketSystemIdentifiers` is a repeated string field that directly corresponds to the "Assignees" in the ticketing system. 

For instance, in Slack, identifiers are not usernames or emails, but unique strings like `U03CS3FK54Z`. Therefore, this field should be configured based on the specifics of your ticketing system.

### Quick Population:

I'm not recommending this for production, but if you are just testing you can use the following query to help populate the table for testing:
```
truncate table `your_dataset.recommender_routing_table`;
INSERT INTO `your_dataset.recommender_routing_table` (Target, ProjectID, TicketSystemIdentifiers)
SELECT 'TicketTestChannel' AS Target, project_id AS ProjectID, ['TicketSystemIdentifier'] AS TicketSystemIdentifiers
FROM (SELECT DISTINCT project_id FROM `your_dataset.flattened_recommendations`);
INSERT INTO `your_dataset.recommender_routing_table` (Target, ProjectID, TicketSystemIdentifiers)
SELECT 'TicketTestChannel' AS Target, NULL AS ProjectID, ['TicketSystemIdentifier'] AS TicketSystemIdentifiers
UNION ALL
SELECT 'TicketTestChannel' AS Target, "" AS ProjectID, ['TicketSystemIdentifier'] AS TicketSystemIdentifiers;

```

## License

This project is licensed under the Apache License - see the [LICENSE.md](LICENSE.md) file for details
