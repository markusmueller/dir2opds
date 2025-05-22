package search

import "encoding/xml"

// OpenSearchDefinition See https://github.com/dewitt/opensearch/blob/master/opensearch-1-1-draft-6.md
type OpenSearchDefinition struct {
	XMLName        xml.Name      `xml:"http://a9.com/-/spec/opensearch/1.1/ OpenSearchDescription"`
	InputEncoding  string        `xml:"InputEncoding"`
	OutputEncoding string        `xml:"OutputEncoding"`
	OpenSearchUrl  OpenSearchUrl `xml:"Url"`
}
