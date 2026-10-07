package web

import (
	"fmt"
	"html"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Redirect is a private, short-lived HTML file that sends the browser to the
// keyed URL. clawsh web opens the file instead of the URL itself, so the access
// key never appears on a command line, where other local users could read it
// (e.g. /proc/<pid>/cmdline on Linux). Jupyter does the same.
type Redirect struct {
	Dir  string // private directory (0700) holding the file
	Path string // the file (0600)
}

// WriteRedirect writes a page that immediately navigates to target, with a
// link as a fallback, into a new private temporary directory.
func WriteRedirect(target string) (*Redirect, error) {
	dir, err := os.MkdirTemp("", "clawsh-web-") // created 0700
	if err != nil {
		return nil, err
	}
	r := &Redirect{Dir: dir, Path: filepath.Join(dir, "open.html")}
	f, err := os.OpenFile(r.Path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	_, err = fmt.Fprint(f, redirectPage(target))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	return r, nil
}

func redirectPage(target string) string {
	u := html.EscapeString(target)
	return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="referrer" content="no-referrer">
<meta http-equiv="refresh" content="0;url=` + u + `">
<title>clawsh</title>
</head>
<body>
<p>Opening clawsh… If nothing happens, <a href="` + u + `">open clawsh</a>.</p>
</body>
</html>
`
}

// URL is the file:// URL of the page, the argument handed to the browser
// opener. It never contains the access key.
func (r *Redirect) URL() string {
	p := filepath.ToSlash(r.Path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p // Windows: C:/... becomes file:///C:/...
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

// Remove deletes the file and its directory. It is safe to call more than once.
func (r *Redirect) Remove() error {
	return os.RemoveAll(r.Dir)
}
