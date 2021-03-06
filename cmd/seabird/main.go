package main // import "github.com/belak/go-seabird/cmd/seabird"

import (
	"math/rand"
	"os"
	"time"

	"github.com/sirupsen/logrus"

	// Officially supported DB drivers
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	// Load plugins
	_ "github.com/belak/go-seabird-plugins/core/all"
	_ "github.com/belak/go-seabird-plugins/extra/all"
	_ "github.com/belak/go-seabird-plugins/url/all"

	// Load the core
	"github.com/belak/go-seabird"
)

func failIfErr(err error, desc string) {
	if err != nil {
		logrus.WithError(err).Fatalln(desc)
	}
}

func main() {
	// Seed the random number generator for plugins to use.
	rand.Seed(time.Now().UTC().UnixNano())

	conf := os.Getenv("SEABIRD_CONFIG")
	if conf == "" {
		conf = "config.toml"
		_, err := os.Stat(conf)
		failIfErr(err, "Failed to load config")
	}

	confReader, err := os.Open(conf)
	failIfErr(err, "Failed to load config")

	// Create the bot
	b, err := seabird.NewBot(confReader)
	failIfErr(err, "Failed to create new bot")

	// Run the bot
	err = b.ConnectAndRun()
	failIfErr(err, "Failed to create run bot")
}
