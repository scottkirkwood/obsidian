package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	dirFlag    = flag.String("dir", "~/zk/Zettelkasten/books", "Folder to place .md files")
	inFileFlag = flag.String("in", "My Clippings.txt", "File containing highlights and notes")
)

type Clipping struct {
	title     string
	page      int
	start     int
	end       int
	date      time.Time
	highlight string
}

type Clippings []Clipping

// Conf is the configurations information for this tool
type Conf struct {
	inputFile string
	outputDir string

	clippings Clippings

	// key is shorttitle, (author) is the filename
	existing map[string][]string
}

var (
	// Ex. "- Your Highlight on page 190 | Location 1870-1871 | Added on Friday, April 15, 2022 12:24:16 AM"

	rxLocation = regexp.MustCompile(`- Your Highlight on page (\d+) \| Location (\d+)\-(\d+) \| Added on (.*)`)
)

const (
	horzLine    = "=========="
	clipsHeader = "## Highlights\n"
)

func (c *Conf) Read(fname string) error {
	c.clippings = nil
	file, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var lastClipping Clipping
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		// For some bizarre reason, Titles often start with BOM Unicode marker
		// Let's remove that
		line := strings.Trim(scanner.Text(), "\ufeff")
		if line == horzLine {
			c.clippings = append(c.clippings, lastClipping)
			lastClipping.title = ""
		} else if lastClipping.title == "" {
			lastClipping.title = line
		} else if rxLocation.MatchString(line) {
			matches := rxLocation.FindStringSubmatch(line)
			lastClipping.page = toInt(matches[1], lineNo)
			lastClipping.start = toInt(matches[2], lineNo)
			lastClipping.end = toInt(matches[3], lineNo)
			lastClipping.date = toDate(matches[4], lineNo)
		} else {
			lastClipping.highlight = line
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func toInt(txt string, lineNo int) int {
	num, err := strconv.Atoi(txt)
	if err != nil {
		fmt.Printf("Problem parsing number %q at line %d\n", txt, lineNo)
	}
	return num
}

func toDate(txt string, lineNo int) time.Time {
	t, err := time.Parse("Monday, January 2, 2006 3:04:05 PM", txt)
	if err != nil {
		fmt.Printf("Problem parsing date %q at line %d\n", txt, lineNo)
	}
	return t
}

func createConf(inFile, outputDir string) *Conf {
	outputDir = strings.ReplaceAll(outputDir, "~", "$HOME")
	outputDir = os.ExpandEnv(outputDir)
	return &Conf{
		inputFile: inFile,
		outputDir: outputDir,
	}
}

func (c *Conf) LookupExisting() error {
	c.existing = map[string][]string{}
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
		title, author := fields["title"], fields["author"]
		c.addExisting(title, author, fname)
		if strings.Contains(title, "–") {
			split := strings.Split(title, "–")
			if len(split[0]) > 10 {
				c.addExisting(split[0], author, fname)
			}
		}
		c.addExisting(fields["short_title"], author, fname)
	}
	return nil
}

func (c *Conf) addExisting(title, author, fname string) {
	title = strings.TrimSpace(title)
	author = strings.TrimSpace(author)
	fname = strings.TrimSpace(fname)
	key := fmt.Sprintf("%s (%s)", title, author)
	if title == "" {
		key = fmt.Sprintf("(%s)", author)
	} else if author == "" {
		key = title
	}
	c.existing[key] = append(c.existing[key], fname)
}

func (c *Conf) findFiles(clip Clipping) ([]string, error) {
	fnames, ok := c.existing[clip.title]
	if !ok {
		fmt.Printf("Not found %q\n", clip.title)
	}
	return fnames, nil
}

func (c *Conf) UpdateExisting() error {
	fileToClippings := map[string][]Clipping{}
	for _, clip := range c.clippings {
		files, err := c.findFiles(clip)
		if err != nil {
			return err
		}
		if len(files) > 0 {
			if len(files) == 1 {
				fname := files[0]
				fileToClippings[fname] = append(fileToClippings[fname], clip)
			}
		}
	}
	for fname, clips := range fileToClippings {
		if err := c.updateFile(fname, clips); err != nil {
			return err
		}
	}
	return nil
}

func (c *Conf) updateFile(fname string, clips []Clipping) error {
	lines := make([]string, 0, len(clips))
	for _, clip := range clips {
		lines = append(lines, clip.makeTextBlock())
	}
	return updateFileWithText(fname, clipsHeader, strings.Join(lines, "\n"))
}

func (c Clipping) makeTextBlock() string {
	return fmt.Sprintf("- Page: %d Pos: %d-%d Date: %s\n> %s\n", c.page, c.start, c.end, c.date.Format("2006-01-02"), c.highlight)
}

func updateFileWithText(fname, header, txt string) error {
	lines, err := readLines(fname)
	if err != nil {
		return err
	}
	hasHeader := strings.Contains(strings.Join(lines, "\n"), header)
	if hasHeader {
		// replace
		fmt.Printf("Updating %q\n", fname)
		lines = removeHeaderSection(lines, header)
	} else {
		// append
		fmt.Printf("Appending to %q\n", fname)
	}
	lines = append(lines, header)
	lines = append(lines, txt)
	tmpFilename, err := writeLines(fname, lines)
	if err != nil {
		return err
	}
	return os.Rename(tmpFilename, fname)
}

func removeHeaderSection(lines []string, header string) []string {
	newLines := make([]string, 0, len(lines))
	inHeader := false
	header = strings.TrimSpace(header)
	for _, line := range lines {
		if inHeader {
			if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "---") {
				inHeader = false
			}
		} else {
			if line == header {
				inHeader = true
				continue
			}
			newLines = append(newLines, line)
		}
	}
	return newLines
}

func readLines(fname string) ([]string, error) {
	lines := []string{}
	file, err := os.Open(fname)
	if err != nil {
		return lines, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, nil
}

func writeLines(fname string, lines []string) (string, error) {
	tmpFilename := fname + ".tmp"
	file, err := os.Create(tmpFilename)
	if err != nil {
		return tmpFilename, err
	}
	defer file.Close()
	_, err = file.WriteString(strings.Join(lines, "\n"))
	return tmpFilename, err
}

var (
	keyString = regexp.MustCompile(`([^:]+): "(.*)"`)
	keyVal    = regexp.MustCompile(`([^:]+): (.*)`)
)

// Note: this also exists in goodreads2obs.go should be refactored
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
	conf := createConf(*inFileFlag, *dirFlag)
	err := conf.Read(*inFileFlag)
	if err != nil {
		fmt.Printf("Unable to read %q: %v\n", *inFileFlag, err)
		return
	}
	if err := conf.LookupExisting(); err != nil {
		fmt.Printf("Unable to lookup existing: %v\n", err)
		return
	}
	if err := conf.UpdateExisting(); err != nil {
		fmt.Printf("Unable to update existing: %v\n", err)
		return
	}
}
