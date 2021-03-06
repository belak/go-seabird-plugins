package url

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/yhat/scrape"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	seabird "github.com/belak/go-seabird"
	"github.com/belak/go-seabird-plugins/internal"
)

func init() {
	seabird.RegisterPlugin("url", newPlugin)
}

// NOTE: This isn't perfect in any sense of the word, but it's pretty close
// and I don't know if it's worth the time to make it better.
var (
	urlRegex     = regexp.MustCompile(`https?://[^ ]+`)
	newlineRegex = regexp.MustCompile(`\s*\n\s*`)

	contextKeyURLPlugin = internal.ContextKey("seabird-url-plugin")
)

// NOTE: This nasty work is done so we ignore invalid ssl certs. We know what
// we're doing.
//nolint:gosec
var client = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
	Timeout: 5 * time.Second,
}

// LinkProvider is a callback to be registered with the Plugin. It
// takes the same parameters as a normal IRC callback in addition to a
// *url.URL representing the found url. It returns true if it was able
// to handle that url and false otherwise.
type LinkProvider func(r *seabird.Request, url *url.URL) bool

// Plugin stores all registered URL LinkProviders
type Plugin struct {
	providers map[string][]LinkProvider
}

func CtxPlugin(ctx context.Context) *Plugin {
	return ctx.Value(contextKeyURLPlugin).(*Plugin)
}

func newPlugin(b *seabird.Bot) error {
	p := &Plugin{
		providers: make(map[string][]LinkProvider),
	}

	bm := b.BasicMux()
	cm := b.CommandMux()

	bm.Event("PRIVMSG", p.callback)

	cm.Event("down", isItDownCallback, &seabird.HelpInfo{
		Usage:       "<website>",
		Description: "Checks if given website is down",
	})

	b.SetValue(contextKeyURLPlugin, p)

	return nil
}

// RegisterProvider registers a LinkProvider for a specific domain.
func (p *Plugin) RegisterProvider(domain string, f LinkProvider) error {
	p.providers[domain] = append(p.providers[domain], f)

	return nil
}

func (p *Plugin) callback(r *seabird.Request) {
	for _, rawurl := range urlRegex.FindAllString(r.Message.Trailing(), -1) {
		go func(raw string) {
			u, err := url.ParseRequestURI(raw)
			if err != nil {
				return
			}

			// Strip the last character if it's a slash
			u.Path = strings.TrimRight(u.Path, "/")

			for _, provider := range p.providers[u.Host] {
				if provider(r, u) {
					return
				}
			}

			// If there was a www, we fall back to no www
			// This is not perfect, but it will fix a number of issues
			// Alternatively, we could require the linkifiers to
			// register multiple times
			if strings.HasPrefix(u.Host, "www.") {
				host := strings.TrimPrefix(u.Host, "www.")
				for _, provider := range p.providers[host] {
					if provider(r, u) {
						return
					}
				}
			}

			defaultLinkProvider(raw, r)
		}(rawurl)
	}
}

func defaultLinkProvider(url string, r *seabird.Request) bool {
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false
	}

	// We search the first 1K and if a title isn't in there, we deal with it
	z, err := html.Parse(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		r.GetLogger("url").WithError(err).Warn("Failed to grab URL")
		return false
	}

	// Scrape the tree for the first title node we find
	n, ok := scrape.Find(z, scrape.ByTag(atom.Title))

	// If we got a result, pull the text from it
	if ok {
		title := newlineRegex.ReplaceAllLiteralString(scrape.Text(n), " ")
		r.Replyf("Title: %s", title)
	}

	return ok
}

func isItDownCallback(r *seabird.Request) {
	go func() {
		url, err := url.Parse(r.Message.Trailing())
		if err != nil {
			r.Replyf("URL doesn't appear to be valid")
			return
		}

		if url.Scheme == "" {
			url.Scheme = "http"
		}

		resp, err := client.Head(url.String())
		if err == nil {
			defer resp.Body.Close()
		}

		if err != nil || resp.StatusCode != 200 {
			r.Replyf("It's not just you! %s looks down from here.", url)
			return
		}

		r.Replyf("It's just you! %s looks up from here!", url)
	}()
}
