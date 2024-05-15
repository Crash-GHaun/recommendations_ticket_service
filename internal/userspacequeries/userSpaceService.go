package userspacequeries

import (
	"io/ioutil"
	"path/filepath"

	u "ticketservice/internal/utils"
)

var (
	EnabledQueries map[string]string
)

func loadQueries(directory string) (map[string]string, error) {
	files, err := filepath.Glob(filepath.Join(directory, "*.tpl"))
	if err != nil {
		return nil, err
	}

	queries := make(map[string]string)
	for _, file := range files {
		content, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, err
		}

		queryName := filepath.Base(file)
		queries[queryName] = string(content)
	}

	return queries, nil
}

func InitUserSpaceQueries() error {
	var err error
	queryDirectory := "./internal/userspacequeries/queries"
	EnabledQueries, err = loadQueries(queryDirectory)
	if err != nil {
		return err
	}

	// Print loaded queries
	for name, query := range EnabledQueries {
		u.LogPrint(1, "Query Name: %s\nQuery Content:\n%s\n", name, query)
	}
	return nil
}
