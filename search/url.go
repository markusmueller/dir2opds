package search

import "encoding/xml"

type OpenSearchUrl struct {
	XMLName  xml.Name `xml:"Url"`
	Type     string   `xml:"type,attr"`
	Template string   `xml:"template,attr"`
}
