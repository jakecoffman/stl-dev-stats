package main

import (
	"github.com/jakecoffman/stldevs/aggregator"
	"github.com/jakecoffman/stldevs/config"
	"github.com/jakecoffman/stldevs/web"
	"github.com/jmoiron/sqlx"
	"io"
	"log"
	"os"
	"runtime"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	setupLogger("log.txt")
	f, err := os.Open("config.json")
	if err != nil {
		log.Fatal(err)
	}

	cfg, err := config.NewConfig(f)
	if err != nil {
		log.Fatal(err)
	}

	db, err := sqlx.Connect("mysql", "root:"+cfg.MysqlPw+"@/stldevs?parseTime=true")
	if err != nil {
		log.Fatal(err)
	}
	db.MapperFunc(config.CamelToSnake)

	agg := aggregator.New(db, cfg.GithubKey)

	web.Run(cfg, db, agg)
}

func setupLogger(fileName string) {
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalln("Failed to open log file", err)
	}
	log.SetOutput(io.MultiWriter(os.Stderr, file))
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
