package web

import (
	"html"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestRedirectFile(t *testing.T) {
	const key = "0123456789abcdef0123456789abcdef"
	target := "http://127.0.0.1:7421/?k=" + key + "&x=<\">"
	r, err := WriteRedirect(target)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Remove()

	b, err := os.ReadFile(r.Path)
	if err != nil {
		t.Fatal(err)
	}
	page := string(b)
	esc := html.EscapeString(target)
	if !strings.Contains(page, `content="0;url=`+esc+`"`) {
		t.Errorf("no meta refresh to the escaped target:\n%s", page)
	}
	if !strings.Contains(page, `href="`+esc+`"`) {
		t.Errorf("no fallback link to the escaped target:\n%s", page)
	}
	if strings.Contains(page, target) {
		t.Errorf("target is not escaped:\n%s", page)
	}
	if strings.Contains(r.Path, key) {
		t.Errorf("file name carries the key: %s", r.Path)
	}

	u := r.URL()
	if !strings.HasPrefix(u, "file:///") {
		t.Errorf("URL = %q, want a file:/// URL", u)
	}
	if strings.Contains(u, key) {
		t.Errorf("the opener argument carries the key: %s", u)
	}

	if runtime.GOOS != "windows" {
		fi, err := os.Stat(r.Path)
		if err != nil {
			t.Fatal(err)
		}
		if m := fi.Mode().Perm(); m != 0o600 {
			t.Errorf("file mode = %o, want 600", m)
		}
		di, err := os.Stat(r.Dir)
		if err != nil {
			t.Fatal(err)
		}
		if m := di.Mode().Perm(); m != 0o700 {
			t.Errorf("dir mode = %o, want 700", m)
		}
	}

	if err := r.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(r.Dir); !os.IsNotExist(err) {
		t.Errorf("dir still there after Remove: %v", err)
	}
	if err := r.Remove(); err != nil {
		t.Errorf("second Remove: %v", err)
	}
}

func TestRedirectURLWindowsPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("drive-letter paths only resolve on Windows")
	}
	r := &Redirect{Path: `C:\Users\Me Too\AppData\Local\Temp\ccm-web-1\open.html`}
	want := "file:///C:/Users/Me%20Too/AppData/Local/Temp/ccm-web-1/open.html"
	if got := r.URL(); got != want {
		t.Errorf("URL = %q, want %q", got, want)
	}
}
