// Package scrape helps write code to scrape a page.
package scrape

import (
	"bufio"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/hermanschaaf/prettyprint"
	"golang.org/x/net/publicsuffix" // go get golang.org/x/net/publicsuffix
)

const (
	// NormalTimeout is how long we should wait before refetching from site
	NormalTimeout = time.Duration(time.Minute * 2)
	// NoCache will not use the cases (timeout is zero)
	NoCache      = time.Duration(time.Minute * 0)
	cookieTimout = time.Duration(time.Hour * 5)
	// UA is the user Agent we will be using
	UA             = `Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/69.0.3497.100 Safari/537.36`
	cookieJarFname = "/tmp/scrape-cookies.txt"
	splitHead      = "-------- HEAD --------"
	splitBody      = "-------- BODY --------"
)

var (
	reCrLfs     = regexp.MustCompile("(\r\n)+")
	reLfSpaceLf = regexp.MustCompile("\n(\\s*\n)+")
)

// Conn is the basic connection object.
type Conn struct {
	// LoginURL should be set before calling Login.
	LoginURL       string
	CookieJarFname string
	Verbose        int

	// FailedLogin should be set for a function that returns true if login may have timed out or failed.
	FailedLogin func(content string) bool

	// DontCache should be used for page results that shouldn't be cached
	DontCache func(content string) bool

	CacheNameFmt string
	UserName     string
	Password     string

	jar *cookiejar.Jar
}

// NewConn creates a new connection
func NewConn() *Conn {
	return &Conn{
		CookieJarFname: cookieJarFname,
		Verbose:        1,
		FailedLogin:    defaultTimedOut,
		DontCache:      defaultDontCache,
		CacheNameFmt:   "/tmp/scrape-%x.html",
	}
}

// Login goes to the login url with a username and password.
func (c *Conn) Login() error {
	contents, err := c.PostURL(c.LoginURL, map[string]string{
		"id":       "submit",
		"userId":   c.UserName,
		"password": c.Password,
	})
	if err != nil {
		return err
	}
	if c.FailedLogin(contents) {
		return fmt.Errorf("unable to login")
	}
	return nil
}

// ConfigFromNetRc gets some configuration information from the ~/.netrc file.
func (c *Conn) ConfigFromNetRc(machine string) error {
	usr, err := user.Current()
	if err != nil {
		return err
	}
	username, password, err := readNetRc(filepath.Join(usr.HomeDir, ".netrc"), machine)
	if err != nil {
		if c.Verbose > 1 {
			fmt.Fprintf(os.Stderr, "Unable to read ~/.netrc: %v\n", err)
		}
		return err
	}
	c.UserName = username
	c.Password = password
	return nil
}

// FetchAndCache fetches and url and caches it for later.
func (c *Conn) FetchAndCache(uri string, expireDuration time.Duration) (header, contents string, fromCache bool, err error) {
	if c.Verbose > 0 {
		fmt.Fprintf(os.Stderr, "Fetching: %q\n", uri)
	}
	fromCache = true
	header, contents, err = c.fetchFromCache(uri, expireDuration)
	if err != nil {
		if c.Verbose > 0 {
			fmt.Fprintf(os.Stderr, "Problem fetching from cache: %v\n", err)
		}
		fromCache = false
		header, contents, err = c.fetchURL(uri)
		if err != nil {
			return header, contents, fromCache, err
		}
	}
	if c.FailedLogin(contents) {
		return header, contents, fromCache, fmt.Errorf("%q site timed out", uri)
	}
	return header, contents, fromCache, nil
}

var dateRx = regexp.MustCompile(`Date:\[([^\]]+)\]`)

// ParseDate helps parse a date from HTTP server
func ParseDate(header string) (time.Time, error) {
	date := dateRx.FindStringSubmatch(header)
	if len(date) <= 1 {
		return time.Unix(0, 0), fmt.Errorf("unable to parse for date %q", header)
	}
	return http.ParseTime(date[1])
}

// PostURL posts and url with the given map.
func (c *Conn) PostURL(uri string, fields map[string]string) (string, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Jar:     c.getJar(),
	}
	vals := url.Values{}
	for k, v := range fields {
		vals.Add(k, v)
	}
	if c.Verbose > 1 {
		fmt.Fprintf(os.Stderr, "Posting %q\n", uri)
	}
	req, err := http.NewRequest("POST", uri, strings.NewReader(vals.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", UA)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Get the resp body as a string
	dataInBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return string(dataInBytes), err
	}
	contents := c.prettyContents(string(dataInBytes))
	c.cache(uri, resp, contents) // ignore caching errors
	if resp.StatusCode/100 != 2 {
		return contents, fmt.Errorf("status code: %d for %q", resp.StatusCode, uri)
	}
	return contents, c.saveCookies(uri)
}

func (c *Conn) saveCookies(uri string) error {
	u, err := url.Parse(uri)
	if err != nil {
		return err
	}
	f, err := os.Create(c.CookieJarFname)
	if err != nil {
		return err
	}
	defer f.Close()
	// Note: we could also save, merge hostnames
	fmt.Fprintf(f, "[%s://%s]\n", u.Scheme, u.Host)
	for _, cookie := range c.jar.Cookies(u) {
		// Note: Not storing any of the other values (bad)
		fmt.Fprintf(f, "%s:%s:%s\n", cookie.Name, cookie.Value, time.Now().Format(time.RFC3339))
	}
	if c.Verbose > 2 {
		fmt.Fprintf(os.Stderr, "saveCookies %q %q\n", c.CookieJarFname, uri)
	}
	return nil
}

func (c *Conn) newCookies() (err error) {
	c.jar, err = cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil || c.jar == nil {
		if c.Verbose > 0 {
			fmt.Fprintf(os.Stderr, "Failed to create cookie jar: %v\n", err)
		}
		return
	}
	f, err := os.Open(c.CookieJarFname)
	if err != nil {
		if c.Verbose > 2 {
			fmt.Fprintf(os.Stderr, "Ignoring error %v\n", err)
		}
		err = nil
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var u *url.URL
	for scanner.Scan() {
		line := scanner.Text()
		l := len(line)
		if l > 0 && line[0] == '[' && line[l-1] == ']' {
			uri := line[1 : l-1]
			u, err = url.Parse(uri)
			if err != nil {
				if c.Verbose > 0 {
					fmt.Fprintf(os.Stderr, "Error parsing uri %q: %v", uri, err)
				}
				return
			}
		} else if u != nil {
			cols := strings.SplitN(line, ":", 3)
			d, err := time.Parse(time.RFC3339, cols[2])
			if len(cols) != 3 {
				if c.Verbose > 0 {
					fmt.Fprintf(os.Stderr, "Unable to parse line %q\n", line)
				}
				continue
			}
			if err != nil {
				if c.Verbose > 0 {
					fmt.Fprintf(os.Stderr, "Unable to parse %q: %v\n", cols[2], err)
				}
				continue
			} else if time.Now().Sub(d) > cookieTimout {
				if c.Verbose > 1 {
					fmt.Fprintf(os.Stderr, "Cookies too old %v\n", d)
				}
				continue
			}
			c.jar.SetCookies(u, []*http.Cookie{&http.Cookie{
				Name:  cols[0],
				Value: cols[1],
			}})
		}
	}
	return nil
}

func (c *Conn) getJar() *cookiejar.Jar {
	if err := c.newCookies(); err != nil {
		if c.Verbose > 0 {
			fmt.Fprintf(os.Stderr, "Problem getting jar: %v\n", err)
		}
	}
	return c.jar
}

func readNetRc(filename, machine string) (username, password string, err error) {
	file, err := os.Open(filename)
	if err != nil {
		return
	}
	defer file.Close()
	s := bufio.NewScanner(file)
	capture := false
	for s.Scan() {
		line := s.Text()
		if len(line) > 0 && line[0] == '#' {
			continue
		}
		fields := strings.SplitN(line, " ", 2)
		if len(fields) != 2 {
			continue
		}
		if !capture {
			if fields[0] == "machine" && fields[1] == machine {
				capture = true
			}
		} else {
			switch fields[0] {
			case "login":
				username = fields[1]
			case "password":
				password = fields[1]
			}
			if username != "" && password != "" {
				return
			}
		}
	}
	err = s.Err()
	return
}

func (c *Conn) fetchURL(uri string) (header, contents string, err error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Jar:     c.getJar(),
	}
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", UA)

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	header = fmt.Sprintf("%s", resp.Header)

	// Get the resp body as a string
	dataInBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return header, string(dataInBytes), err
	}
	contents = c.prettyContents(string(dataInBytes))
	c.cache(uri, resp, contents) // ignore caching errors
	return header, contents, c.saveCookies(uri)
}

func (c *Conn) cacheName(uri string) string {
	h := md5.New()
	io.WriteString(h, uri)
	return fmt.Sprintf(c.CacheNameFmt, h.Sum(nil))
}

func (c *Conn) cache(uri string, resp *http.Response, contents string) error {
	if c.DontCache(contents) {
		// don't cache useless pages.
		return nil
	}
	fname := c.cacheName(uri)
	if err := ioutil.WriteFile(fname, cacheFormat(uri, resp, contents), 0644); err != nil {
		if c.Verbose > 0 {
			fmt.Fprintf(os.Stderr, "Error writing cache %v\n", err)
		}
		return err
	}
	if c.Verbose > 1 {
		fmt.Fprintf(os.Stderr, "\nWrote %s\n", fname)
	}
	return nil
}

func (c *Conn) prettyContents(contents string) string {
	pretty, err := prettyprint.Prettify(contents, "  ")
	if err != nil {
		if c.Verbose > 0 {
			fmt.Fprintf(os.Stderr, "Not XML %s\n", err)
		}
		pretty = contents
	}
	pretty = strings.TrimSpace(pretty)
	pretty = reCrLfs.ReplaceAllString(pretty, "\n")
	pretty = reLfSpaceLf.ReplaceAllString(pretty, "\n")
	return pretty
}

// fetchFromCache returns the header, contents and maybe an error
func (c *Conn) fetchFromCache(uri string, expireDuration time.Duration) (string, string, error) {
	fname := c.cacheName(uri)
	info, err := os.Stat(fname)
	if err != nil {
		return "", "", err
	}
	ago := time.Now().Sub(info.ModTime())
	if ago < expireDuration {
		bytes, err := ioutil.ReadFile(fname)
		header, err := headerFromCache(bytes)
		if err != nil {
			return header, "", err
		}
		contents, err := textFromCache(bytes)
		if err != nil {
			return header, contents, err
		}
		if c.FailedLogin(contents) {
			return header, contents, fmt.Errorf("cached a timed out page %s", fname)
		}
		if c.Verbose > 1 {
			fmt.Printf("Using cache %q, ago %v\n", fname, ago)
		}
		return header, contents, err
	}
	return "", "", fmt.Errorf("%q cache too old %v", fname, ago)
}

func cacheFormat(uri string, resp *http.Response, contents string) []byte {
	return []byte(fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n", uri, splitHead, resp.Header, splitBody, contents))
}

var rxSplits = regexp.MustCompile(fmt.Sprintf(`(?ms)(.*)\n%s\n(.*)\n%s\n(.*)`, splitHead, splitBody))

func textFromCache(contents []byte) (string, error) {
	parts := rxSplits.FindSubmatch(contents)
	if len(parts) != 4 {
		return string(parts[0]), fmt.Errorf("did not match for cached text contents")
	}
	return string(parts[3]), nil
}

func headerFromCache(contents []byte) (string, error) {
	parts := rxSplits.FindSubmatch(contents)
	if len(parts) != 4 {
		return string(parts[0]), fmt.Errorf("did not match for cached header")
	}
	return string(parts[2]), nil
}

var rxTimeOut = regexp.MustCompile(`[Tt]imed out`)

func defaultTimedOut(contents string) bool {
	return rxTimeOut.MatchString(contents)
}

func defaultDontCache(contents string) bool {
	return rxTimeOut.MatchString(contents)
}
