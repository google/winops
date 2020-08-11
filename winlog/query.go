// +build windows

package winlog

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"

	"golang.org/x/sys/windows/registry"
)

// QueryList is the root node for the defined Query schema.
// https://msdn.microsoft.com/en-us/library/windows/desktop/aa385678(v=vs.85).aspx
type QueryList struct {
	Select []Select `xml:"Query>Select"`
}

// Select is an XPath query that identifies events to include in the query result set.
// https://msdn.microsoft.com/en-us/library/windows/desktop/aa385766(v=vs.85).aspx
type Select struct {
	Path string `xml:"Path,attr"`
	Text string `xml:",chardata"`
}

// BuildStructuredXMLQuery builds a structured XML query from a map of channels
// and XPath queries based on the expected query schema. Only supports Select XPaths.
// https://msdn.microsoft.com/en-us/library/windows/desktop/aa385760(v=vs.85).aspx
func BuildStructuredXMLQuery(queries map[string]string) ([]byte, error) {
	var q QueryList
	for k, v := range queries {
		q.Select = append(q.Select, Select{Path: k, Text: v})
	}
	xmlQuery, err := xml.Marshal(q)
	if err != nil {
		return nil, fmt.Errorf("xml.Marshal failed: %v", err)
	}
	return xmlQuery, nil
}

// QueryRegConfiguration reads a registry key for channels and XPaths in value data pairs.
func QueryRegConfiguration(regKey registry.Key, path string, maxChannels int) (map[string]string, error) {
	k, err := registry.OpenKey(regKey, path, registry.QUERY_VALUE)
	if err != nil {
		return nil, fmt.Errorf("registry.OpenKey failed: %v", err)
	}
	defer k.Close()

	// Read channel paths.
	channels, err := k.ReadValueNames(maxChannels)
	if err != io.EOF && err != nil {
		return nil, fmt.Errorf("registry.ReadValueNames failed: %v", err)
	}

	// Fill map.
	queries := make(map[string]string)
	for _, channel := range channels {
		xpath, val, err := k.GetStringValue(channel)
		if err != nil {
			// If value isn't a REG_SZ, skip.
			if val != registry.SZ {
				log.Printf("QueryRegConfiguration: unexpected value type (%d) found for %s.", val, channel)
				continue
			}
			// Return all other errors.
			return nil, fmt.Errorf("registry.GetStringValue failed: %v", err)
		}
		queries[channel] = xpath
	}

	return queries, nil
}
