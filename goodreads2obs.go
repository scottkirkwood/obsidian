// goodreads 2 obsidian converter
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gocarina/gocsv"
)

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
}

var ()

func readFile(fname string) ([]*GoodReadCols, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	books := []*GoodReadCols{}

	if err := gocsv.UnmarshalFile(f, &books); err != nil {
		return books, err
	}
	return books, nil
}

func formatBook(t *template.Template, book *GoodReadCols) error {
	fname := makeFilename(book.Title)
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

func makeDirs(fname string) error {
	dir := filepath.Dir(fname)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}

func makeFilename(fname string) string {
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
	return "books/" + fname + ".mod"
}

func cleanupBook(book *GoodReadCols) {
	book.ISBN = strings.Trim(book.ISBN[1:], `"`)
	book.ISBN13 = strings.Trim(book.ISBN13[1:], `"`)
	if book.DateRead == "" {
		book.DateRead = book.DateAdded
	}
	book.DateRead = strings.Replace(book.DateRead, "/", "-", -1)
}

func formatBooks(books []*GoodReadCols) error {
	t, err := template.ParseFiles("book-template.md")
	if err != nil {
		return err
	}
	for _, book := range books {
		cleanupBook(book)
		if err := formatBook(t, book); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	fmt.Printf("Goodreads Converter\n")
	books, err := readFile("goodreads.csv")
	if err != nil {
		fmt.Printf("Error reading file %v\n", err)
	}
	fmt.Printf("NumBooks %d\n", len(books))
	if err := formatBooks(books); err != nil {
		fmt.Printf("Error formatting books %v\n", err)
	}
}
