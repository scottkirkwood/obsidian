package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
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

var (
	// Ex. "- Your Highlight on page 190 | Location 1870-1871 | Added on Friday, April 15, 2022 12:24:16 AM"

	rxLocation = regexp.MustCompile(`- Your Highlight on page (\d+) \| Location (\d+)\-(\d+) \| Added on (.*)`)
	horzLine   = "=========="
)

func Read(fname string) (Clippings, error) {
	c := Clippings{}
	file, err := os.Open(fname)
	if err != nil {
		return c, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var lastClipping Clipping
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if line == horzLine {
			c = append(c, lastClipping)
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
		return c, err
	}
	return c, nil
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

// Conf is the configurations information for this tool
type Conf struct {
	inputFile string
	outputDir string

	// key is shorttitle, (author) is the filename
	existing map[string]string
}

func main() {
	fmt.Println("vim-go")
	c, err := Read(*inFileFlag)
	if err != nil {
		fmt.Printf("Unable to read %q: %v\n", *inFileFlag, err)
		return
	}
	fmt.Printf("%d entries\n", len(c))
}
