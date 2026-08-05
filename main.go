package main

import (
	"database/sql"
	"flag"

	"github.com/martinlehoux/biking_home/cli"
	"github.com/martinlehoux/biking_home/config"
	"github.com/martinlehoux/kagamigo/kcore"
	_ "github.com/mattn/go-sqlite3"
)

var (
	configFile = flag.String("config", "config.yaml", "path to the YAML configuration file")
)

func main() {
	flag.Parse()
	appConfig, err := config.Load(*configFile)
	kcore.Expect(err, "failed to load configuration")
	db, err := sql.Open("sqlite3", appConfig.Database.Path)
	kcore.Expect(err, "failed to open database")
	defer db.Close()
	cli.Run(db, *configFile, appConfig)
}
