//go:build system

package systemtest

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// assetRefRe finds the hashed bundle the built index.html points at. The name
// changes on every build, so the test discovers it instead of hardcoding one.
var assetRefRe = regexp.MustCompile(`/(assets/[^"']+\.(?:js|css))`)

// The regression test for the release pipeline. The UI lives in a gitignored
// directory that go:embed reads at compile time, so a build that skips the vite
// step produces a binary with an empty filesystem. The web module rejects that at
// startup, which means a broken artifact cannot even reach this assertion — but
// when it does start, this is what proves the SPA actually shipped inside it.
func TestServerServesTheEmbeddedUI(t *testing.T) {
	s := startServer(t)

	res, body := s.get("/", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET / answered %s, want 200", res.Status)
	}

	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("GET / served Content-Type %q, want text/html", ct)
	}

	page := string(body)
	if !strings.Contains(page, `<div id="root"`) {
		t.Fatalf("GET / did not return the SPA document:\n%s", truncate(page))
	}

	// A document with no bundle reference is a build that produced markup but no
	// JavaScript, which would leave a blank page in the browser
	if !assetRefRe.MatchString(page) {
		t.Fatalf("the served index.html references no hashed asset:\n%s", truncate(page))
	}
}

// The hashed assets are the ones safe to cache forever, and the server says so.
// index.html must not carry the same promise or a deploy never reaches anybody.
func TestEmbeddedAssetsCarryCacheHeaders(t *testing.T) {
	s := startServer(t)

	_, index := s.get("/", nil)

	match := assetRefRe.FindStringSubmatch(string(index))
	if match == nil {
		t.Fatalf("no asset reference in the served index.html:\n%s", truncate(string(index)))
	}
	asset := "/" + match[1]

	res, _ := s.get(asset, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s answered %s, want 200", asset, res.Status)
	}

	if cc := res.Header.Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("GET %s served Cache-Control %q, want an immutable policy", asset, cc)
	}

	etag := res.Header.Get("ETag")
	if etag == "" {
		t.Fatalf("GET %s served no ETag", asset)
	}

	indexRes, _ := s.get("/", nil)
	if cc := indexRes.Header.Get("Cache-Control"); strings.Contains(cc, "immutable") {
		t.Errorf("index.html served Cache-Control %q, which would pin clients to one release", cc)
	}

	// The ETag is only worth serving if it also shortcuts the next request
	revalidated, body := s.get(asset, map[string]string{"If-None-Match": etag})
	if revalidated.StatusCode != http.StatusNotModified {
		t.Errorf("a conditional GET %s answered %s, want 304", asset, revalidated.Status)
	}
	if len(body) != 0 {
		t.Errorf("the 304 for %s carried %d bytes of body", asset, len(body))
	}
}

// Client-side routes only exist in the browser, so a reload of one has to come
// back as the SPA document rather than a 404.
func TestDeepLinksFallBackToTheSPA(t *testing.T) {
	s := startServer(t)

	navigation := map[string]string{
		"Sec-Fetch-Mode": "navigate",
		"Accept":         "text/html,application/xhtml+xml",
	}

	for _, path := range []string{"/tasks", "/recipes/12", "/settings/users"} {
		res, body := s.get(path, navigation)
		if res.StatusCode != http.StatusOK {
			t.Errorf("GET %s answered %s, want 200", path, res.Status)
			continue
		}

		if !strings.Contains(string(body), `<div id="root"`) {
			t.Errorf("GET %s did not return the SPA document", path)
		}
	}
}

// A missing asset is a missing asset. Answering those with index.html too would
// hand a fetch() an HTML body where it expected JSON or JavaScript.
func TestMissingAssetsAreNotSwallowedByTheFallback(t *testing.T) {
	s := startServer(t)

	res, _ := s.get("/assets/does-not-exist.js", map[string]string{
		"Sec-Fetch-Mode": "no-cors",
	})

	if res.StatusCode != http.StatusNotFound {
		t.Errorf("GET a missing asset answered %s, want 404", res.Status)
	}
}

// The UI and the API share an origin, so the fallback must never shadow the API
// tree: an unknown API path is a 404 from the router, not the SPA document.
func TestUnknownAPIRoutesDoNotReturnHTML(t *testing.T) {
	s := startServer(t)

	res, body := s.get("/api/v1/not-a-route", map[string]string{
		"Accept": "text/html",
	})

	if res.StatusCode != http.StatusNotFound {
		t.Errorf("an unknown API route answered %s, want 404", res.Status)
	}

	if strings.Contains(string(body), `<div id="root"`) {
		t.Error("an unknown API route returned the SPA document")
	}
}

func truncate(s string) string {
	const max = 800
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n...[truncated]"
}
