// package service provides a http handler that reads the path in the request.url and returns
// an xml document that follows the OPDS 1.1 standard
// https://specs.opds.io/opds-1.1.html
package service

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dubyte/dir2opds/search"

	"github.com/dubyte/dir2opds/opds"
	"golang.org/x/tools/blog/atom"
)

func init() {
	_ = mime.AddExtensionType(".mobi", "application/x-mobipocket-ebook")
	_ = mime.AddExtensionType(".epub", "application/epub+zip")
	_ = mime.AddExtensionType(".cbz", "application/x-cbz")
	_ = mime.AddExtensionType(".cbr", "application/x-cbr")
	_ = mime.AddExtensionType(".fb2", "text/fb2+xml")
	_ = mime.AddExtensionType(".pdf", "application/pdf")
}

const (
	pathTypeFile = iota
	pathTypeDirOfDirs
	pathTypeDirOfFiles
)

const (
	ignoreFile       = true
	includeFile      = false
	currentDirectory = "."
	parentDirectory  = ".."
	hiddenFilePrefix = "."
)

type OPDS struct {
	TrustedRoot      string
	HideCalibreFiles bool
	UseCalibreCovers bool
	HideDotFiles     bool
	NoCache          bool
}

type IsDirer interface {
	IsDir() bool
}

func isFile(e IsDirer) bool {
	return !e.IsDir()
}

const navigationType = "application/atom+xml;profile=opds-catalog;kind=navigation"
const acquisitionType = "application/atom+xml;profile=opds-catalog;kind=acquisition"

const searchType = "application/opensearchdescription+xml"
const searchDefinitionPath = "/" + searchDefinitionName
const searchDefinitionName = "opensearch.xml"
const searchPath = "/search"

var TimeNow = timeNowFunc()

// Handler serve the content of a book file or
// returns an Acquisition Feed when the entries are documents or
// returns a Navigation Feed when the entries are other folders
func (s OPDS) Handler(w http.ResponseWriter, req *http.Request) error {
	var err error
	urlPath, err := url.PathUnescape(req.URL.Path)
	if err != nil {
		log.Printf("error while serving '%s': %s", req.URL.Path, err)
		return err
	}

	if urlPath == searchDefinitionPath {
		var content []byte

		searchDefinition := &search.OpenSearchDefinition{
			InputEncoding:  "UTF-8",
			OutputEncoding: "UTF-8",
			OpenSearchUrl:  search.OpenSearchUrl{Type: "application/atom+xml;profile=opds-catalog;kind=acquisition", Template: "/search?q={searchTerms}"},
		}

		content, err = xml.MarshalIndent(searchDefinition, "  ", "    ")
		content = append([]byte(xml.Header), content...)

		w.Header().Add("Content-Type", "application/xml")

		http.ServeContent(w, req, searchDefinitionName, TimeNow(), bytes.NewReader(content))
		return nil
	} else if urlPath == "/" {
		var content []byte
		navigation := s.makeFeedRoot(req)
		content, err = xml.MarshalIndent(navigation, "  ", "    ")
		content = append([]byte(xml.Header), content...)
		w.Header().Add("Content-Type", navigationType)
		http.ServeContent(w, req, "feed.xml", TimeNow(), bytes.NewReader(content))
		return nil
	} else if urlPath == "/new" {
		var content []byte
		navigation := s.makeFeedNewest(req)
		content, err = xml.MarshalIndent(navigation, "  ", "    ")
		content = append([]byte(xml.Header), content...)
		w.Header().Add("Content-Type", navigationType)
		http.ServeContent(w, req, "feed.xml", TimeNow(), bytes.NewReader(content))
		return nil
	}

	var query = ""
	var fPath string
	if urlPath == searchPath {
		query = req.URL.Query().Get("q")

		if query == "" {
			return errors.New("query param 'q' empty or missing")
		}
		fPath = s.TrustedRoot
	}

	if strings.HasPrefix(urlPath, "/shelf") {
		// remove prefix /shelf
		fPath = filepath.Join(s.TrustedRoot, strings.Replace(urlPath, "/shelf", "/", 1))
	}

	// verifyPath avoid the http transversal by checking the path is under DirRoot
	_, err = verifyPath(fPath, s.TrustedRoot)
	if err != nil {
		log.Printf("fPath %q err: %s", fPath, err)
		w.WriteHeader(http.StatusNotFound)
		return nil
	}

	log.Printf("urlPath:'%s'", urlPath)

	if _, err := os.Stat(fPath); err != nil {
		log.Printf("fPath err: %s", err)
		w.WriteHeader(http.StatusNotFound)
		return err
	}

	log.Printf("fPath:'%s'", fPath)

	// it's a file just serve the file
	if getPathType(fPath) == pathTypeFile {
		_, pathRelativeToContentRoot, _ := strings.Cut(fPath, s.TrustedRoot+"/")
		if s.UseCalibreCovers && strings.HasSuffix(pathRelativeToContentRoot, "cover.jpg") {
			http.ServeFile(w, req, fPath)
		}
		if fileShouldBeIgnored(pathRelativeToContentRoot, s.HideCalibreFiles, s.HideDotFiles) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.Header().Add("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(pathRelativeToContentRoot)))
			http.ServeFile(w, req, fPath)
		}
		return nil
	}

	if s.NoCache {
		w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Add("Expires", "0")
	}

	var content []byte

	if urlPath == searchPath {
		searchResult, size := s.makeFeedSearchResult(req, query)
		acFeed := &search.SearchResultFeed{Feed: &searchResult, Size: size, OS: "http://purl.org/dc/terms/", Opds: "http://opds-spec.org/2010/catalog", Dc: "http://purl.org/dc/terms/"}
		content, err = xml.MarshalIndent(acFeed, "  ", "    ")
		w.Header().Add("Content-Type", "application/atom+xml;profile=opds-catalog;kind=acquisition")
	} else if getPathType(fPath) == pathTypeDirOfFiles {
		navFeed := s.makeFeedPath(fPath, req)
		acFeed := &opds.AcquisitionFeed{Feed: &navFeed, Dc: "http://purl.org/dc/terms/", Opds: "http://opds-spec.org/2010/catalog"}
		content, err = xml.MarshalIndent(acFeed, "  ", "    ")
		w.Header().Add("Content-Type", "application/atom+xml;profile=opds-catalog;kind=acquisition")
	} else { // it is a navigation feed
		navFeed := s.makeFeedPath(fPath, req)
		content, err = xml.MarshalIndent(navFeed, "  ", "    ")
		w.Header().Add("Content-Type", "application/atom+xml;profile=opds-catalog;kind=navigation")
	}

	if err != nil {
		log.Printf("error while serving '%s': %s", fPath, err)
		return err
	}

	content = append([]byte(xml.Header), content...)
	http.ServeContent(w, req, "feed.xml", TimeNow(), bytes.NewReader(content))

	return nil
}
func (s OPDS) makeFeedRoot(req *http.Request) atom.Feed {
	newestContent := atom.Text{Type: "text", Body: "The 15 latest modified books, most-recently-modified first."}
	allContent := atom.Text{Type: "text", Body: "All books."}

	feedBuilder := opds.FeedBuilder.
		ID(req.URL.Path).
		Title("Home").
		Updated(TimeNow()).
		AddLink(opds.LinkBuilder.Rel("start").Href("/").Type(navigationType).Build()).
		AddLink(opds.LinkBuilder.Rel("search").Href(searchDefinitionPath).Type(searchType).Build())

	var builder = opds.EntryBuilder{}

	builder = opds.EntryBuilder{}.Title("Newest books").ID("/new").AddLink(opds.LinkBuilder.Href("/new").Rel("http://opds-spec.org/sort/new").Type(acquisitionType).Build()).Content(&newestContent)

	feedBuilder = feedBuilder.AddEntry(builder.Build())

	builder = opds.EntryBuilder{}.Title("All books").ID("/shelf").AddLink(opds.LinkBuilder.Href("/shelf").Rel("http://opds-spec.org/subsection").Type(acquisitionType).Build()).Content(&allContent)

	feedBuilder = feedBuilder.AddEntry(builder.Build())

	return feedBuilder.Build()
}

func (s OPDS) makeFeedPath(fpath string, req *http.Request) atom.Feed {
	feedBuilder := opds.FeedBuilder.
		ID(req.URL.Path).
		Title("Catalog in " + req.URL.Path).
		Updated(TimeNow()).
		AddLink(opds.LinkBuilder.Rel("start").Href("/").Type(navigationType).Build()).
		AddLink(opds.LinkBuilder.Rel("search").Href(searchDefinitionPath).Type(searchType).Build())

	dirEntries, _ := os.ReadDir(fpath)
	for _, entry := range dirEntries {
		if fileShouldBeIgnored(entry.Name(), s.HideCalibreFiles, s.HideDotFiles) {
			continue
		}

		pathType := getPathType(filepath.Join(fpath, entry.Name()))

		var builder = opds.EntryBuilder{}

		rel := getRel(entry.Name(), pathType)

		builder = builder.ID(filepath.Join(req.URL.Path, entry.Name())).
			Title(entry.Name()).
			AddLink(opds.LinkBuilder.
				Rel(rel).
				Title(entry.Name()).
				Href(filepath.Join(req.URL.RequestURI(), url.PathEscape(entry.Name()))).
				Type(getType(entry.Name(), pathType)).
				Build())

		if rel == "http://opds-spec.org/acquisition" {
			builder = addCoverIfExists(filepath.Join(fpath, entry.Name()), builder, s)
		}

		feedBuilder = feedBuilder.
			AddEntry(builder.Build())
	}
	return feedBuilder.Build()
}

type File struct {
	filePath string
	fileInfo os.FileInfo
}

func (s OPDS) makeFeedNewest(req *http.Request) atom.Feed {
	feedBuilder := search.FeedBuilder.
		ID(req.URL.Path).
		Title("Newest books").
		Updated(TimeNow()).
		AddLink(opds.LinkBuilder.Rel("start").Href("/").Type(navigationType).Build()).
		AddLink(opds.LinkBuilder.Rel("search").Href(searchDefinitionPath).Type(searchType).Build())

	var files = []File{}

	filepath.WalkDir(s.TrustedRoot, func(path string, file fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		_, pathRelativeToContentRoot, _ := strings.Cut(path, s.TrustedRoot+"/")

		if file.IsDir() && fileShouldBeIgnored(pathRelativeToContentRoot, s.HideCalibreFiles, s.HideDotFiles) {
			return filepath.SkipDir
		}

		if !file.IsDir() {
			if fileShouldBeIgnored(file.Name(), s.HideCalibreFiles, s.HideDotFiles) {
				// skip
			} else {
				if getPathType(path) == pathTypeFile {
					info, _ := file.Info()
					files = append(files, File{filePath: path, fileInfo: info})
				}
			}
		}
		return nil
	})

	// sorting files by modified descending
	sort.Slice(files, func(i, j int) bool {
		fileI := files[i].fileInfo
		fileJ := files[j].fileInfo

		if !fileI.ModTime().Equal(fileJ.ModTime()) {
			return fileI.ModTime().After(fileJ.ModTime())
		}

		return fileI.Name() < fileJ.Name()
	})

	for i := 0; i < 14 && i < len(files); i++ {
		file := files[i]
		_, pathRelativeToContentRoot, _ := strings.Cut(file.filePath, s.TrustedRoot+"/")

		var builder = opds.EntryBuilder{}

		builder = builder.ID(filepath.Join("/shelf", pathRelativeToContentRoot)).
			Title(file.fileInfo.Name()).
			AddLink(opds.LinkBuilder.
				Rel("http://opds-spec.org/acquisition").
				Title(file.fileInfo.Name()).
				Href(filepath.Join("/shelf", url.PathEscape(pathRelativeToContentRoot))).
				Type(getType(file.fileInfo.Name(), pathTypeFile)).
				Build())

		builder = addCoverIfExists(file.filePath, builder, s)

		feedBuilder = feedBuilder.
			AddEntry(builder.Build())
	}

	return feedBuilder.Build()
}

func (s OPDS) makeFeedSearchResult(req *http.Request, query string) (atom.Feed, int) {
	feedBuilder := search.FeedBuilder.
		ID(req.URL.Path).
		Title(fmt.Sprintf("Folders containing files matching query %s", query)).
		Updated(TimeNow()).
		AddLink(opds.LinkBuilder.Rel("start").Href("/").Type(navigationType).Build()).
		AddLink(opds.LinkBuilder.Rel("search").Href(searchDefinitionPath).Type(searchType).Build())

	var count = 0
	filepath.WalkDir(s.TrustedRoot, func(path string, file fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		_, pathRelativeToContentRoot, _ := strings.Cut(path, s.TrustedRoot+"/")

		if file.IsDir() && fileShouldBeIgnored(pathRelativeToContentRoot, s.HideCalibreFiles, s.HideDotFiles) {
			return filepath.SkipDir
		}

		if !file.IsDir() {
			if fileShouldBeIgnored(pathRelativeToContentRoot, s.HideCalibreFiles, s.HideDotFiles) {
				// skip
			} else {
				if strings.Contains(strings.ToLower(file.Name()), strings.ToLower(query)) {
					var builder = opds.EntryBuilder{}

					builder = builder.
						ID(filepath.Join("/shelf", pathRelativeToContentRoot)).
						Title(file.Name()).
						AddLink(opds.LinkBuilder.
							Rel(getRel(file.Name(), 0)).
							Href(filepath.Join("/shelf", url.PathEscape(pathRelativeToContentRoot))).
							Type(getType(file.Name(), 0)).
							Build())

					builder = addCoverIfExists(path, builder, s)

					feedBuilder = feedBuilder.AddEntry(builder.Build())
					count++
				}
			}
		}
		return nil
	})
	return feedBuilder.Build(), count
}

func fileShouldBeIgnored(filename string, hideCalibreFiles, hideDotFiles bool) bool {
	// not ignore those directories
	if filename == currentDirectory || filename == parentDirectory {
		return includeFile
	}

	if hideDotFiles && strings.HasPrefix(filename, hiddenFilePrefix) {
		return ignoreFile
	}

	if hideCalibreFiles &&
		(strings.Contains(filename, ".opf") ||
			strings.Contains(filename, "cover.") ||
			strings.Contains(filename, "metadata.db") ||
			strings.Contains(filename, "metadata_db_prefs_backup.json") ||
			strings.Contains(filename, ".caltrash") ||
			strings.Contains(filename, ".calnotes")) {
		return ignoreFile
	}

	return false
}

func getRel(name string, pathType int) string {
	if pathType == pathTypeDirOfFiles || pathType == pathTypeDirOfDirs {
		return "subsection"
	}

	ext := filepath.Ext(name)
	if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" {
		return "http://opds-spec.org/image/thumbnail"
	}

	// mobi, epub, etc
	return "http://opds-spec.org/acquisition"
}

func getType(name string, pathType int) string {
	switch pathType {
	case pathTypeFile:
		return mime.TypeByExtension(filepath.Ext(name))
	case pathTypeDirOfFiles:
		return acquisitionType
	case pathTypeDirOfDirs:
		return navigationType
	default:
		return mime.TypeByExtension("xml")
	}
}

func getPathType(dirpath string) int {
	fi, err := os.Stat(dirpath)
	if err != nil {
		log.Printf("getPathType os.Stat err: %s", err)
	}

	if isFile(fi) {
		return pathTypeFile
	}

	dirEntries, err := os.ReadDir(dirpath)
	if err != nil {
		log.Printf("getPathType: readDir err: %s", err)
	}

	for _, entry := range dirEntries {
		if isFile(entry) {
			return pathTypeDirOfFiles
		}
	}
	// Directory of directories
	return pathTypeDirOfDirs
}

func timeNowFunc() func() time.Time {
	t := time.Now()
	return func() time.Time { return t }
}

// verify path use a trustedRoot to avoid http transversal
// from https://www.stackhawk.com/blog/golang-path-traversal-guide-examples-and-prevention/
func verifyPath(path, trustedRoot string) (string, error) {
	// clean is already used upstream but leaving this
	// to keep the functionality of the function as close as possible to the blog.
	c := filepath.Clean(path)

	// get the canonical path
	r, err := filepath.EvalSymlinks(c)
	if err != nil {
		fmt.Println("Error " + err.Error())
		return c, errors.New("unsafe or invalid path specified")
	}

	if !inTrustedRoot(r, trustedRoot) {
		return r, errors.New("unsafe or invalid path specified")
	}

	return r, nil
}

func inTrustedRoot(path string, trustedRoot string) bool {
	return strings.HasPrefix(path, trustedRoot)
}

func addCoverIfExists(akquisitionPath string, builder opds.EntryBuilder, s OPDS) opds.EntryBuilder {
	if s.UseCalibreCovers {
		coverPath := filepath.Dir(akquisitionPath) + "/cover.jpg"
		stat, err := os.Stat(coverPath)

		if err == nil {
			_, coverPathRelativeToContentRoot, _ := strings.Cut(coverPath, s.TrustedRoot+"/")

			builder = builder.AddLink(opds.LinkBuilder.
				Rel("http://opds-spec.org/image").
				Href(filepath.Join("/shelf", url.PathEscape(coverPathRelativeToContentRoot))).
				Type(getType(stat.Name(), pathTypeFile)).
				Build())
		}
	}

	return builder
}
