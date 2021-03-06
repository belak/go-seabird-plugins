package karma

import (
	"regexp"
	"strings"
	"unicode"

	"xorm.io/xorm"

	seabird "github.com/belak/go-seabird"
	"github.com/belak/go-seabird-plugins/extra/db"
)

func init() {
	seabird.RegisterPlugin("karma", newKarmaPlugin)
}

type karmaPlugin struct {
	db *xorm.Engine
}

// Karma represents an item with a karma count
type Karma struct {
	ID    int64
	Name  string `xorm:"unique"`
	Score int
}

var karmaRegex = regexp.MustCompile(`([\w]{2,}|".+?")(\+\++|--+)(?:\s|$)`)

func newKarmaPlugin(b *seabird.Bot) error {
	if err := b.EnsurePlugin("db"); err != nil {
		return err
	}

	p := &karmaPlugin{
		db: db.CtxDB(b.Context()),
	}

	// Migrate any relevant tables
	err := p.db.Sync(Karma{})
	if err != nil {
		return err
	}

	bm := b.BasicMux()
	cm := b.CommandMux()

	cm.Event("karma", p.karmaCallback, &seabird.HelpInfo{
		Usage:       "<nick>",
		Description: "Displays karma for given user",
	})

	bm.Event("PRIVMSG", p.callback)

	return nil
}

func (p *karmaPlugin) cleanedName(name string) string {
	return strings.TrimFunc(strings.ToLower(name), unicode.IsSpace)
}

// GetKarmaFor returns the karma for the given name.
func (p *karmaPlugin) GetKarmaFor(name string) int {
	out := &Karma{Name: p.cleanedName(name)}

	// Note that we're explicitly ignoring an error here because it's not a
	// problem when this returns zero results.
	_, _ = p.db.Get(out)

	return out.Score
}

// UpdateKarma will update the karma for a given name and return the new karma value.
func (p *karmaPlugin) UpdateKarma(name string, diff int) int {
	out := &Karma{Name: p.cleanedName(name)}

	p.db.Transaction(func(s *xorm.Session) (interface{}, error) {
		found, _ := s.Get(out)
		if !found {
			s.Insert(out)
		}
		out.Score += diff
		return s.ID(out.ID).Cols("score").Update(out)
	})

	return out.Score
}

func (p *karmaPlugin) karmaCallback(r *seabird.Request) {
	term := strings.TrimSpace(r.Message.Trailing())

	// If we don't provide a term, search for the current nick
	if term == "" {
		term = r.Message.Prefix.Name
	}

	r.MentionReplyf("%s's karma is %d", term, p.GetKarmaFor(term))
}

func (p *karmaPlugin) callback(r *seabird.Request) {
	if len(r.Message.Params) < 2 || !r.FromChannel() {
		return
	}

	var buzzkillTriggered bool

	var jerkModeTriggered bool

	var changes = make(map[string]int)

	matches := karmaRegex.FindAllStringSubmatch(r.Message.Trailing(), -1)
	for _, v := range matches {
		// If it starts with a ", we know it also ends with a quote so we
		// can chop them off.
		if strings.HasPrefix(v[1], "\"") {
			v[1] = v[1][1 : len(v[1])-1]
		}

		diff := len(v[2]) - 1
		cleanedName := p.cleanedName(v[1])
		cleanedNick := p.cleanedName(r.Message.Prefix.Name)

		// If it's negative, or positive and someone is trying to change
		// their own karma we need to reverse the sign.
		if v[2][0] == '-' || cleanedName == cleanedNick {
			diff *= -1
		}

		changes[v[1]] += diff
	}

	for name, diff := range changes {
		if diff > 5 {
			buzzkillTriggered = true
			diff = 5
		}

		if diff < -5 {
			jerkModeTriggered = true
			diff = -5
		}

		r.Replyf("%s's karma is now %d", name, p.UpdateKarma(name, diff))
	}

	if buzzkillTriggered {
		r.Replyf("Buzzkill Mode (tm) enforced a maximum karma change of 5")
	}

	if jerkModeTriggered {
		r.Replyf("Don't Be a Jerk Mode (tm) enforced a maximum karma change of 5")
	}
}
