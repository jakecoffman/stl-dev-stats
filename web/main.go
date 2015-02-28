package web

import (
	"log"
	"net/http"

	"text/template"
	"time"

	"database/sql"

	"encoding/gob"

	"github.com/google/go-github/github"
	"github.com/gorilla/context"
	"github.com/gorilla/sessions"
	"github.com/jakecoffman/stldevs/aggregator"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/oauth2"
)

const (
	base = "web"
)

var store = sessions.NewFilesystemStore("", []byte("secret")) // TODO

func Run() {
	db, err := sql.Open("mysql", "root:bird@/stldevs")
	if err != nil {
		log.Println(err)
		return
	}
	defer db.Close()
	agg := aggregator.New(db)
	if time.Since(agg.LastRun()) > 12*time.Hour {
		agg.Run()
	}

	gob.Register(github.User{})

	fileHandler := http.FileServer(http.Dir(base + "/static/"))

	router := httprouter.New()
	router.GET("/static/*filepath", handleFiles(fileHandler))
	router.GET("/oauth2", oauth2Handler)
	router.GET("/logout", logout)
	router.GET("/", index)
	router.GET("/toplangs", topLangs(agg))
	router.GET("/profile", profile(agg))
	router.NotFound = http.HandlerFunc(notFound)
	router.PanicHandler = panicHandler

	log.Println("Serving on", "localhost:80")
	log.Println(http.ListenAndServe("localhost:80", context.ClearHandler(router)))
}

func handleFiles(fileServer http.Handler) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		r.URL.Path = p.ByName("filepath")
		fileServer.ServeHTTP(w, r)
	}
}

func notFound(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, base+"/static/404.html")
}

func panicHandler(w http.ResponseWriter, r *http.Request, d interface{}) {
	template, err := template.ParseGlob(base + "/templates/*.html")
	if err != nil {
		log.Println(err)
		return
	}

	if err = template.ExecuteTemplate(w, "error", d); err != nil {
		log.Println(err)
	}
}

func index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	data := commonSessionData(w, r)
	parseAndExecute(w, "index", data)
}

func topLangs(agg *aggregator.Aggregator) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		data := commonSessionData(w, r)
		data["langs"] = agg.PopularLanguages()
		data["lastrun"] = agg.LastRun().Local().Format("Jan 2, 2006 at 3:04pm")
		parseAndExecute(w, "toplangs", data)
	}
}

func profile(agg *aggregator.Aggregator) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		data := commonSessionData(w, r)
		parseAndExecute(w, "profile", data)
	}
}

// TODO in production we want to just parse once
func parseAndExecute(w http.ResponseWriter, templateName string, data interface{}) {
	template, err := template.ParseGlob(base + "/templates/*.html")
	if err != nil {
		panic(err)
	}
	if err = template.ExecuteTemplate(w, templateName, data); err != nil {
		panic(err)
	}
}

func commonSessionData(w http.ResponseWriter, r *http.Request) map[string]interface{} {
	data := map[string]interface{}{}
	user, _ := get_session(r, "user")
	if user != nil {
		data["user"] = user
		// TODO extract an admin list
		if *user.(github.User).Login == "jakecoffman" {
			data["admin"] = true
		}
	} else {
		state := randSeq(10)
		set_session(w, r, "state", state)
		data["github"] = conf.AuthCodeURL(state, oauth2.AccessTypeOffline)
	}
	return data
}
