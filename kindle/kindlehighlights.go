package main

import (
	"fmt"
	scrape "github.com/scottkirkwood/obsidian"
)

// LoginToSite logs into the site
func LoginToSite(c *scrape.Conn) error {
	if err := c.ConfigFromNetRc("read.amazon.com"); err != nil {
		return err
	}
	fmt.Printf("user: %s\n", c.UserName)
	c.LoginURL = "https://read.amazon.com/notebook"
	if err := c.Login(); err != nil {
		return err
	}
	return nil
}

func main() {
	c := scrape.NewConn()
	c.Verbose = 1
	LoginToSite(c)
}
