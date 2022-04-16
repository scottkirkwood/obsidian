// goodreads 2 obsidian converter
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/gocarina/gocsv"
)

var (
	dirFlag          = flag.String("dir", "~/zk/Zettelkasten/books", "Folder to place .md files")
	inFileFlag       = flag.String("in", "goodreads.csv", "File containing goodreads info in csv format")
	templateFileFlag = flag.String("template", "book-template.md", "Template to use")
)

// Conf is the configurations information for this tool
type Conf struct {
	inputFile    string
	outputDir    string
	templateFile string

	tempDir string
	books   []*GoodReadCols

	// Key is either isbn, raw title, or filename
	// value is the filename
	existing map[string]string
}

func newConf(inputFile, outputDir, templateFile string) *Conf {
	outputDir = strings.ReplaceAll(outputDir, "~", "$HOME")
	outputDir = os.ExpandEnv(outputDir)
	return &Conf{
		inputFile:    inputFile,
		outputDir:    outputDir,
		templateFile: templateFile,
		tempDir:      filepath.Join(os.TempDir(), "goodreads"),
	}
}

type moveFile struct {
	fromFile  string
	toFile    string // If empty, we delete fromFile
	different bool   // Found both and they are different
}

type moveFiles []moveFile

type GoodReadCols struct {
	Id                       string `csv:"Book Id"`
	Title                    string `csv:"Title"`
	Author                   string `csv:"Author"`
	AuthorLF                 string `csv:"Author l-f"`
	AdditionalAuthors        string `csv:"Additional Authors"`
	ISBN                     string `csv:"ISBN"`
	ISBN13                   string `csv:"ISBN13"`
	Rating                   string `csv:"My Rating"`
	Average                  string `csv:"Average Rating"`
	Publisher                string `csv:"Publisher"`
	Binding                  string `csv:"Binding"`
	Pages                    string `csv:"Number of Pages"`
	Year                     string `csv:"Year Published"`
	OriginalYear             string `csv:"Original Publication Year"`
	DateRead                 string `csv:"Date Read"`
	DateAdded                string `csv:"Date Added"`
	Bookshelves              string `csv:"Bookshelves"`
	BookshelvesPositions     string `csv:"Bookshelves with positions"`
	ExclusiveShelf           string `csv:"Exclusive Shelf"`
	Review                   string `csv:"My Review"`
	Spoiler                  string `csv:"Spoiler"`
	PrivateNotes             string `csv:"Private Notes"`
	ReadCount                string `csv:"Read Count"`
	RecommendedFor           string `csv:"Recommended For"`
	RecommendedBy            string `csv:"Recommended By"`
	OwnedCopies              string `csv:"Owned Copies"`
	OriginalPurchaseDate     string `csv:"Original Purchase Date"`
	OriginalPurchaseLocation string `csv:"Original Purchase Location"`
	Condition                string `csv:"Condition"`
	ConditionDescription     string `csv:"Condition Description"`
	BCID                     string `csv:"BCID"`
	Tags                     string
}

func (c *Conf) ReadCSV() ([]*GoodReadCols, error) {
	f, err := os.Open(c.inputFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	books := []*GoodReadCols{}

	if err := gocsv.UnmarshalFile(f, &books); err != nil {
		return books, err
	}
	c.books = books
	return books, nil
}

func (c *Conf) writeBook(t *template.Template, book *GoodReadCols) error {
	fname := c.makeTempFilename(book.Title)
	cleanupBook(book)
	if err := makeDirs(fname); err != nil {
		return err
	}
	f, err := os.Create(fname)
	if err != nil {
		return err
	}
	defer f.Close()
	return t.Execute(f, book)
}

// Make the parent directories for `fname` if it does not already exist
func makeDirs(fname string) error {
	dir := filepath.Dir(fname)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}

func (c *Conf) makeTempFilename(fname string) string {
	idx := strings.Index(fname, ": ")
	if idx > 10 {
		fname = fname[:idx]
	}
	fname = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', '*', '|', '"', '<', '>', '=':
			return '-'
		}
		return r
	}, fname)
	return filepath.Join(c.tempDir, fname+".md")
}

func cleanupBook(book *GoodReadCols) {
	book.Title = strings.ReplaceAll(book.Title, "#", `\#`)
	book.ISBN = strings.Trim(book.ISBN[1:], `"`)
	book.ISBN13 = strings.Trim(book.ISBN13[1:], `"`)
	if book.Rating == "0" {
		book.Rating = ""
	}
	if book.DateRead == "" {
		book.DateRead = book.DateAdded
	}
	tags := []string{"book"}
	for _, bookshelf := range strings.Split(book.Bookshelves, ",") {
		tag := strings.TrimSpace(bookshelf)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	book.Tags = strings.Join(tags, ", ")
	book.DateRead = strings.Replace(book.DateRead, "/", "-", -1)
}

func (c *Conf) WriteBooks() error {
	t, err := template.ParseFiles(c.templateFile)
	if err != nil {
		return err
	}
	fmt.Printf("Temporary output to %s\n", c.tempDir)
	for _, book := range c.books {
		if err := c.writeBook(t, book); err != nil {
			return err
		}
	}
	return nil
}

func (c *Conf) LookupExisting() error {
	c.existing = map[string]string{}
	glob := filepath.Join(c.outputDir, "*.md")
	files, err := filepath.Glob(glob)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		fmt.Printf("Note: no files found for %q\n", glob)
	}
	for _, fname := range files {
		fields, err := readMdFile(fname)
		if err != nil {
			fmt.Printf("Unable to read %s, %v\n", fname, err)
			continue
		}
		isbn := getISBNOrEquivalent(fields, filepath.Base(fname))
		c.existing[isbn] = fname
	}
	return nil
}

func getISBNOrEquivalent(fields map[string]string, baseName string) string {
	isbn, ok := fields["isbn"]
	if !ok {
		title, ok := fields["title"]
		if !ok {
			return baseName
		}
		return title
	}
	return isbn
}

// CompareDirs compares the tempDir with outputDir
// It returns a list of moveFile it things are required
// to make them the same
func (c *Conf) CompareDirs() (moveFiles, error) {
	ret := moveFiles{}

	glob := filepath.Join(c.tempDir, "*.md")
	tmpFiles, err := filepath.Glob(glob)
	if err != nil {
		return ret, err
	}
	for _, tmpFile := range tmpFiles {
		fields, err := readMdFile(tmpFile)
		if err != nil {
			fmt.Printf("Unable to read %q: %v\n", tmpFile, err)
			continue
		}
		newBasename := filepath.Base(tmpFile)
		isbn := getISBNOrEquivalent(fields, newBasename)
		existingName, ok := c.existing[isbn]
		if !ok {
			// it's a new file
			ret = append(ret, moveFile{
				fromFile: tmpFile,
				toFile:   filepath.Join(c.outputDir, newBasename),
			})
		} else {
			equal, err := mdFilesEquivalent(tmpFile, existingName)
			if err != nil {
				fmt.Printf("Error comparing files %q ?= %q: %v\n", tmpFile, existingName, err)
				continue
			}
			if !equal {
				// They are different overwrite
				ret = append(ret, moveFile{
					fromFile:  tmpFile,
					toFile:    existingName,
					different: true,
				})
			} else if filepath.Base(existingName) != newBasename {
				// They are identical, but have different names
				ret = append(ret, moveFile{
					fromFile: tmpFile,
					toFile:   existingName,
				})
			} else {
				// Remove source file
				ret = append(ret, moveFile{
					fromFile: tmpFile,
				})
			}
		}
	}
	return ret, nil
}

// mdFilesEquivalent returns true if they are mostly the same
func mdFilesEquivalent(tmpFile, existingFile string) (bool, error) {
	newBytes, err := os.ReadFile(tmpFile)
	if err != nil {
		return true, err
	}
	newBytes = removeRandomInfo(newBytes)
	oldBytes, err := os.ReadFile(existingFile)
	if err != nil {
		return true, err
	}
	oldBytes = removeRandomInfo(oldBytes)
	return bytes.Equal(newBytes, oldBytes), nil
}

var (
	ratingRx = regexp.MustCompile("average: [^\n]+")
	pagesRx  = regexp.MustCompile("pages: [^\n]+")
)

// removeRandomInfo modifies the bytes of the md file so that some lines
// that change more often (like average ratings) is removed.
func removeRandomInfo(bytes []byte) []byte {
	return pagesRx.ReplaceAll(ratingRx.ReplaceAll(bytes, []byte{}), []byte{})
}

func (c *Conf) Summary(m moveFiles) {
	deleteFileCount, moveFileCount, diffFileCount := 0, 0, 0
	for _, mf := range m {
		if mf.toFile == "" {
			deleteFileCount++
		} else if mf.different {
			diffFileCount++
		} else {
			moveFileCount++
		}
	}
	if moveFileCount+diffFileCount == 0 {
		fmt.Printf("Directories are identical\n")
	} else {
		if moveFileCount > 0 {
			fmt.Printf("Copying %d files\n", moveFileCount)
		}
		if diffFileCount > 0 {
			fmt.Printf("Found %d differences\n", diffFileCount)
			for _, mf := range m {
				if mf.different {
					fmt.Printf("meld %q %q\n", mf.fromFile, mf.toFile)
				}
			}
		}
	}
}

func (m moveFiles) DeleteTempfiles() error {
	for _, mf := range m {
		if mf.toFile == "" {
			if err := os.Remove(mf.fromFile); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m moveFiles) MoveTempFiles() error {
	for _, mf := range m {
		if mf.toFile != "" && !mf.different {
			if err := os.Rename(mf.fromFile, mf.toFile); err != nil {
				err = crossDeviceMove(mf.fromFile, mf.toFile)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func crossDeviceMove(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}

	out, err := os.Create(to)
	if err != nil {
		in.Close()
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	in.Close()
	return os.Remove(from)
}

var (
	keyString = regexp.MustCompile(`([^:]+): "(.*)"`)
	keyVal    = regexp.MustCompile(`([^:]+): (.*)`)
)

func readMdFile(fname string) (map[string]string, error) {
	fields := make(map[string]string)
	bytes, err := os.ReadFile(fname)
	if err != nil {
		return fields, err
	}
	lines := strings.Split(string(bytes), "\n")
	for _, line := range lines {
		matches := keyString.FindStringSubmatch(line)
		if len(matches) == 3 {
			fields[matches[1]] = matches[2]
			continue
		}
		matches = keyVal.FindStringSubmatch(line)
		if len(matches) == 3 {
			fields[matches[1]] = matches[2]
		}
	}
	return fields, nil
}

func main() {
	flag.Parse()

	c := newConf(*inFileFlag, *dirFlag, *templateFileFlag)
	fmt.Printf("Goodreads Converter\n")
	books, err := c.ReadCSV()
	if err != nil {
		fmt.Printf("Error reading file %v\n", err)
		return
	}
	fmt.Printf("NumBooks %d\n", len(books))
	if err := c.WriteBooks(); err != nil {
		fmt.Printf("Error formatting books %v\n", err)
		return
	}
	if err := c.LookupExisting(); err != nil {
		fmt.Printf("Error comparing %v\n", err)
		return
	}
	moveFiles, err := c.CompareDirs()
	if err != nil {
		fmt.Printf("Error comparing %v\n", err)
		return
	}
	c.Summary(moveFiles)
	if err := moveFiles.DeleteTempfiles(); err != nil {
		fmt.Printf("Unable to delete %v\n", err)
		return
	}
	if err := moveFiles.MoveTempFiles(); err != nil {
		fmt.Printf("Unable to mv file %v\n", err)
		return
	}
}
