package service_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dubyte/dir2opds/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler(t *testing.T) {
	// pre-setup
	nowFn := service.TimeNow
	defer func() {
		service.TimeNow = nowFn
	}()

	tests := map[string]struct {
		input             string
		want              string
		WantedContentType string
		wantedStatusCode  int
	}{
		"root navigation":                     {input: "/", want: root, WantedContentType: "application/atom+xml;profile=opds-catalog;kind=navigation", wantedStatusCode: 200},
		"newest 15 books":                     {input: "/new", want: newest, WantedContentType: "application/atom+xml;profile=opds-catalog;kind=navigation", wantedStatusCode: 200},
		"feed (dir of dirs )":                 {input: "/shelf", want: all, WantedContentType: "application/atom+xml;profile=opds-catalog;kind=navigation", wantedStatusCode: 200},
		"acquisitionFeed(dir of files)":       {input: "/shelf/mybook", want: acquisitionFeed, WantedContentType: "application/atom+xml;profile=opds-catalog;kind=acquisition", wantedStatusCode: 200},
		"servingAFile":                        {input: "/shelf/mybook/mybook.txt", want: "Fixture", WantedContentType: "text/plain; charset=utf-8", wantedStatusCode: 200},
		"is not serving hidden file":          {input: "/shelf/.Trash/mybook.epub", want: "Fixture", WantedContentType: "text/plain", wantedStatusCode: 404},
		"serving file with spaces":            {input: "/shelf/mybook/mybook%20copy.txt", want: "Fixture", WantedContentType: "text/plain; charset=utf-8", wantedStatusCode: 200},
		"http trasversal vulnerability check": {input: "/shelf/../../../../mybook", want: all, WantedContentType: "application/atom+xml;profile=opds-catalog;kind=navigation", wantedStatusCode: 404},
		"search definition":                   {input: "/opensearch.xml", want: searchDefinition, WantedContentType: "application/xml", wantedStatusCode: 200},
		"search result":                       {input: "/search?q=mybook", want: searchResult, WantedContentType: "application/atom+xml;profile=opds-catalog;kind=acquisition", wantedStatusCode: 200},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// setup
			s := service.OPDS{"testdata", true, true, true, true}
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.input, nil)
			service.TimeNow = func() time.Time {
				return time.Date(2020, 05, 25, 00, 00, 00, 0, time.UTC)
			}

			// act
			err := s.Handler(w, req)
			require.NoError(t, err)

			// post act
			resp := w.Result()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			// verify
			require.Equal(t, tc.wantedStatusCode, resp.StatusCode)
			if tc.wantedStatusCode != http.StatusOK {
				return
			}
			assert.Equal(t, tc.WantedContentType, resp.Header.Get("Content-Type"))

			print(string(body), "\n")

			assert.Equal(t, tc.want, string(body))
		})
	}

}

var root = `<?xml version="1.0" encoding="UTF-8"?>
  <feed xmlns="http://www.w3.org/2005/Atom">
      <title>Home</title>
      <id>/</id>
      <link rel="start" href="/" type="application/atom+xml;profile=opds-catalog;kind=navigation"></link>
      <link rel="search" href="/opensearch.xml" type="application/opensearchdescription+xml"></link>
      <updated>2020-05-25T00:00:00+00:00</updated>
      <entry>
          <title>Newest books</title>
          <id>/new</id>
          <link rel="http://opds-spec.org/sort/new" href="/new" type="application/atom+xml;profile=opds-catalog;kind=acquisition"></link>
          <published></published>
          <updated></updated>
          <content type="text">The 15 latest modified books, most-recently-modified first.</content>
      </entry>
      <entry>
          <title>All books</title>
          <id>/shelf</id>
          <link rel="http://opds-spec.org/shelf" href="/shelf" type="application/atom+xml;profile=opds-catalog;kind=acquisition"></link>
          <published></published>
          <updated></updated>
          <content type="text">All books.</content>
      </entry>
  </feed>`

var newest = `<?xml version="1.0" encoding="UTF-8"?>
  <feed xmlns="http://www.w3.org/2005/Atom">
      <title>Newest books</title>
      <id>/new</id>
      <link rel="start" href="/" type="application/atom+xml;profile=opds-catalog;kind=navigation"></link>
      <link rel="search" href="/opensearch.xml" type="application/opensearchdescription+xml"></link>
      <updated>2020-05-25T00:00:00+00:00</updated>
      <entry>
          <title>mybook.epub</title>
          <id>/shelf/with cover/mybook.epub</id>
          <link rel="http://opds-spec.org/acquisition" href="/shelf/with%20cover%2Fmybook.epub" type="application/epub+zip" title="mybook.epub"></link>
          <link rel="http://opds-spec.org/image" href="/shelf/with%20cover%2Fcover.jpg" type="image/jpeg"></link>
          <published></published>
          <updated></updated>
      </entry>
      <entry>
          <title>nomatch.txt</title>
          <id>/shelf/nomatch/nomatch.txt</id>
          <link rel="http://opds-spec.org/acquisition" href="/shelf/nomatch%2Fnomatch.txt" type="text/plain; charset=utf-8" title="nomatch.txt"></link>
          <published></published>
          <updated></updated>
      </entry>
      <entry>
          <title>mybook copy.epub</title>
          <id>/shelf/mybook/mybook copy.epub</id>
          <link rel="http://opds-spec.org/acquisition" href="/shelf/mybook%2Fmybook%20copy.epub" type="application/epub+zip" title="mybook copy.epub"></link>
          <published></published>
          <updated></updated>
      </entry>
      <entry>
          <title>mybook copy.txt</title>
          <id>/shelf/mybook/mybook copy.txt</id>
          <link rel="http://opds-spec.org/acquisition" href="/shelf/mybook%2Fmybook%20copy.txt" type="text/plain; charset=utf-8" title="mybook copy.txt"></link>
          <published></published>
          <updated></updated>
      </entry>
      <entry>
          <title>mybook.txt</title>
          <id>/shelf/new folder/mybook.txt</id>
          <link rel="http://opds-spec.org/acquisition" href="/shelf/new%20folder%2Fmybook.txt" type="text/plain; charset=utf-8" title="mybook.txt"></link>
          <published></published>
          <updated></updated>
      </entry>
      <entry>
          <title>mybook.epub</title>
          <id>/shelf/mybook/mybook.epub</id>
          <link rel="http://opds-spec.org/acquisition" href="/shelf/mybook%2Fmybook.epub" type="application/epub+zip" title="mybook.epub"></link>
          <published></published>
          <updated></updated>
      </entry>
      <entry>
          <title>mybook.pdf</title>
          <id>/shelf/mybook/mybook.pdf</id>
          <link rel="http://opds-spec.org/acquisition" href="/shelf/mybook%2Fmybook.pdf" type="application/pdf" title="mybook.pdf"></link>
          <published></published>
          <updated></updated>
      </entry>
      <entry>
          <title>mybook.txt</title>
          <id>/shelf/mybook/mybook.txt</id>
          <link rel="http://opds-spec.org/acquisition" href="/shelf/mybook%2Fmybook.txt" type="text/plain; charset=utf-8" title="mybook.txt"></link>
          <published></published>
          <updated></updated>
      </entry>
  </feed>`

var all = `<?xml version="1.0" encoding="UTF-8"?>
  <feed xmlns="http://www.w3.org/2005/Atom">
      <title>Catalog in /shelf</title>
      <id>/shelf</id>
      <link rel="start" href="/" type="application/atom+xml;profile=opds-catalog;kind=navigation"></link>
      <link rel="search" href="/opensearch.xml" type="application/opensearchdescription+xml"></link>
      <updated>2020-05-25T00:00:00+00:00</updated>
      <entry>
          <title>emptyFolder</title>
          <id>/shelf/emptyFolder</id>
          <link rel="subsection" href="/shelf/emptyFolder" type="application/atom+xml;profile=opds-catalog;kind=acquisition" title="emptyFolder"></link>
          <published></published>
          <updated></updated>
      </entry>
      <entry>
          <title>mybook</title>
          <id>/shelf/mybook</id>
          <link rel="subsection" href="/shelf/mybook" type="application/atom+xml;profile=opds-catalog;kind=acquisition" title="mybook"></link>
          <published></published>
          <updated></updated>
      </entry>
      <entry>
          <title>new folder</title>
          <id>/shelf/new folder</id>
          <link rel="subsection" href="/shelf/new%20folder" type="application/atom+xml;profile=opds-catalog;kind=acquisition" title="new folder"></link>
          <published></published>
          <updated></updated>
      </entry>
      <entry>
          <title>nomatch</title>
          <id>/shelf/nomatch</id>
          <link rel="subsection" href="/shelf/nomatch" type="application/atom+xml;profile=opds-catalog;kind=acquisition" title="nomatch"></link>
          <published></published>
          <updated></updated>
      </entry>
      <entry>
          <title>with cover</title>
          <id>/shelf/with cover</id>
          <link rel="subsection" href="/shelf/with%20cover" type="application/atom+xml;profile=opds-catalog;kind=acquisition" title="with cover"></link>
          <published></published>
          <updated></updated>
      </entry>
  </feed>`

var acquisitionFeed = `<?xml version="1.0" encoding="UTF-8"?>
  <feed xmlns="http://www.w3.org/2005/Atom" xmlns:dc="http://purl.org/dc/terms/" xmlns:opds="http://opds-spec.org/2010/catalog">
      <title>Catalog in /shelf/mybook</title>
      <id>/shelf/mybook</id>
      <link rel="start" href="/" type="application/atom+xml;profile=opds-catalog;kind=navigation"></link>
      <link rel="search" href="/opensearch.xml" type="application/opensearchdescription+xml"></link>
      <updated>2020-05-25T00:00:00+00:00</updated>
      <entry>
          <title>mybook copy.epub</title>
          <id>/shelf/mybook/mybook copy.epub</id>
          <link rel="http://opds-spec.org/acquisition" href="/shelf/mybook/mybook%20copy.epub" type="application/epub+zip" title="mybook copy.epub"></link>
          <published></published>
          <updated></updated>
      </entry>
      <entry>
          <title>mybook copy.txt</title>
          <id>/shelf/mybook/mybook copy.txt</id>
          <link rel="http://opds-spec.org/acquisition" href="/shelf/mybook/mybook%20copy.txt" type="text/plain; charset=utf-8" title="mybook copy.txt"></link>
          <published></published>
          <updated></updated>
      </entry>
      <entry>
          <title>mybook.epub</title>
          <id>/shelf/mybook/mybook.epub</id>
          <link rel="http://opds-spec.org/acquisition" href="/shelf/mybook/mybook.epub" type="application/epub+zip" title="mybook.epub"></link>
          <published></published>
          <updated></updated>
      </entry>
      <entry>
          <title>mybook.pdf</title>
          <id>/shelf/mybook/mybook.pdf</id>
          <link rel="http://opds-spec.org/acquisition" href="/shelf/mybook/mybook.pdf" type="application/pdf" title="mybook.pdf"></link>
          <published></published>
          <updated></updated>
      </entry>
      <entry>
          <title>mybook.txt</title>
          <id>/shelf/mybook/mybook.txt</id>
          <link rel="http://opds-spec.org/acquisition" href="/shelf/mybook/mybook.txt" type="text/plain; charset=utf-8" title="mybook.txt"></link>
          <published></published>
          <updated></updated>
      </entry>
  </feed>`

var searchDefinition = `<?xml version="1.0" encoding="UTF-8"?>
  <OpenSearchDescription xmlns="http://a9.com/-/spec/opensearch/1.1/">
      <InputEncoding>UTF-8</InputEncoding>
      <OutputEncoding>UTF-8</OutputEncoding>
      <Url type="application/atom+xml;profile=opds-catalog;kind=acquisition" template="/search?q={searchTerms}"></Url>
  </OpenSearchDescription>`

var searchResult = `<?xml version="1.0" encoding="UTF-8"?>
  <feed xmlns="http://www.w3.org/2005/Atom" xmlns:dc="http://purl.org/dc/terms/" xmlns:opds="http://opds-spec.org/2010/catalog" xmlns:opensearch="http://purl.org/dc/terms/">
      <title>Folders containing files matching query mybook</title>
      <id>/search</id>
      <link rel="start" href="/" type="application/atom+xml;profile=opds-catalog;kind=navigation"></link>
      <link rel="search" href="/opensearch.xml" type="application/opensearchdescription+xml"></link>
      <updated>2020-05-25T00:00:00+00:00</updated>
      <entry>
          <title>mybook copy.epub</title>
          <id>/shelf/mybook/mybook copy.epub</id>
          <link rel="http://opds-spec.org/acquisition" href="/shelf/mybook%2Fmybook%20copy.epub" type="application/epub+zip"></link>
          <published></published>
          <updated></updated>
      </entry>
      <entry>
          <title>mybook copy.txt</title>
          <id>/shelf/mybook/mybook copy.txt</id>
          <link rel="http://opds-spec.org/acquisition" href="/shelf/mybook%2Fmybook%20copy.txt" type="text/plain; charset=utf-8"></link>
          <published></published>
          <updated></updated>
      </entry>
      <entry>
          <title>mybook.epub</title>
          <id>/shelf/mybook/mybook.epub</id>
          <link rel="http://opds-spec.org/acquisition" href="/shelf/mybook%2Fmybook.epub" type="application/epub+zip"></link>
          <published></published>
          <updated></updated>
      </entry>
      <entry>
          <title>mybook.pdf</title>
          <id>/shelf/mybook/mybook.pdf</id>
          <link rel="http://opds-spec.org/acquisition" href="/shelf/mybook%2Fmybook.pdf" type="application/pdf"></link>
          <published></published>
          <updated></updated>
      </entry>
      <entry>
          <title>mybook.txt</title>
          <id>/shelf/mybook/mybook.txt</id>
          <link rel="http://opds-spec.org/acquisition" href="/shelf/mybook%2Fmybook.txt" type="text/plain; charset=utf-8"></link>
          <published></published>
          <updated></updated>
      </entry>
      <entry>
          <title>mybook.txt</title>
          <id>/shelf/new folder/mybook.txt</id>
          <link rel="http://opds-spec.org/acquisition" href="/shelf/new%20folder%2Fmybook.txt" type="text/plain; charset=utf-8"></link>
          <published></published>
          <updated></updated>
      </entry>
      <entry>
          <title>mybook.epub</title>
          <id>/shelf/with cover/mybook.epub</id>
          <link rel="http://opds-spec.org/acquisition" href="/shelf/with%20cover%2Fmybook.epub" type="application/epub+zip"></link>
          <link rel="http://opds-spec.org/image" href="/shelf/with%20cover%2Fcover.jpg" type="image/jpeg"></link>
          <published></published>
          <updated></updated>
      </entry>
      <opensearch:totalResults>7</opensearch:totalResults>
  </feed>`
