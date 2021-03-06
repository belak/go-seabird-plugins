package youtube

import (
	"fmt"
	"net/url"
	"strings"

	duration "github.com/channelmeter/iso8601duration"

	seabird "github.com/belak/go-seabird"
	"github.com/belak/go-seabird-plugins/internal"
	urlPlugin "github.com/belak/go-seabird-plugins/url"
)

func init() {
	seabird.RegisterPlugin("url/youtube", newYoutubeProvider)
}

var youtubePrefix = "[YouTube]"

type youtubePlugin struct {
	Key string
}

// videos was converted using https://github.com/ChimeraCoder/gojson
type ytVideos struct {
	Items []struct {
		ContentDetails struct {
			Caption         string `json:"caption"`
			Definition      string `json:"definition"`
			Dimension       string `json:"dimension"`
			Duration        string `json:"duration"`
			LicensedContent bool   `json:"licensedContent"`
		} `json:"contentDetails"`
		Snippet struct {
			CategoryID           string `json:"categoryId"`
			ChannelID            string `json:"channelId"`
			ChannelTitle         string `json:"channelTitle"`
			Description          string `json:"description"`
			LiveBroadcastContent string `json:"liveBroadcastContent"`
			Localized            struct {
				Description string `json:"description"`
				Title       string `json:"title"`
			} `json:"localized"`
			PublishedAt string `json:"publishedAt"`
			Thumbnails  struct {
				Default struct {
					Height int    `json:"height"`
					URL    string `json:"url"`
					Width  int    `json:"width"`
				} `json:"default"`
				High struct {
					Height int    `json:"height"`
					URL    string `json:"url"`
					Width  int    `json:"width"`
				} `json:"high"`
				Medium struct {
					Height int    `json:"height"`
					URL    string `json:"url"`
					Width  int    `json:"width"`
				} `json:"medium"`
			} `json:"thumbnails"`
			Title string `json:"title"`
		} `json:"snippet"`
	} `json:"items"`
}

func newYoutubeProvider(b *seabird.Bot) error {
	err := b.EnsurePlugin("url")
	if err != nil {
		return err
	}

	urlPlugin := urlPlugin.CtxPlugin(b.Context())

	// Get API key from seabird config
	yp := &youtubePlugin{}
	if err := b.Config("youtube", yp); err != nil {
		return err
	}

	// Listen for youtube.com and youtu.be URLs
	urlPlugin.RegisterProvider("youtube.com", yp.Handle)
	urlPlugin.RegisterProvider("youtu.be", yp.Handle)

	return nil
}

func (yp *youtubePlugin) Handle(r *seabird.Request, req *url.URL) bool {
	// Get the Video ID from the URL
	p, _ := url.ParseQuery(req.RawQuery)

	var id string

	if len(p["v"]) > 0 {
		// using full www.youtube.com/?v=bbq
		id = p["v"][0]
	} else {
		// using short youtu.be/bbq
		path := strings.Split(req.Path, "/")
		if len(path) < 1 {
			return false
		}
		id = path[1]
	}

	// Get video duration and title
	time, title := getVideo(id, yp.Key)

	// Invalid video ID or no results
	if time == "" && title == "" {
		return false
	}

	// Send out the IRC message
	msg := fmt.Sprintf("%s ~ %s", time, title)
	r.Replyf("%s %s", youtubePrefix, msg)

	return true
}

func getVideo(id string, key string) (time string, title string) {
	// Build the API call
	api := fmt.Sprintf("https://www.googleapis.com/youtube/v3/videos?part=contentDetails%%2Csnippet&id=%s&fields=items(contentDetails%%2Csnippet)&key=%s", id, key)

	var videos ytVideos
	if err := internal.GetJSON(api, &videos); err != nil {
		return "", ""
	}

	// Make sure we found a video
	if len(videos.Items) < 1 {
		return "", ""
	}

	v := videos.Items[0]

	switch v.Snippet.LiveBroadcastContent {
	case "live", "upcoming":
		return strings.Title(v.Snippet.LiveBroadcastContent), v.Snippet.Title
	}

	// Convert duration from ISO8601
	d, err := duration.FromString(v.ContentDetails.Duration)
	if err != nil {
		return "", ""
	}

	var dr string

	// Print Days and Hours only if they're not 0
	//nolint:gocritic
	if d.Days > 0 {
		dr = fmt.Sprintf("%02d:%02d:%02d:%02d", d.Days, d.Hours, d.Minutes, d.Seconds)
	} else if d.Hours > 0 {
		dr = fmt.Sprintf("%02d:%02d:%02d", d.Hours, d.Minutes, d.Seconds)
	} else {
		dr = fmt.Sprintf("%02d:%02d", d.Minutes, d.Seconds)
	}

	return dr, v.Snippet.Title
}
