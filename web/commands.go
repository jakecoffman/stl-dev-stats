package web

import (
	"database/sql"
	"github.com/go-sql-driver/mysql"
	"github.com/google/go-github/github"
	"github.com/jakecoffman/stldevs"
	"log"
	"time"
)

const pageSize = 20

type DBReader interface {
	Select(dest interface{}, query string, args ...interface{}) error
	Get(dest interface{}, query string, args ...interface{}) error
}

func LastRun(db DBReader) *time.Time {
	timeStr := mysql.NullTime{}
	err := db.Get(&timeStr, queryLastRun)
	if err == sql.ErrNoRows {
		return &time.Time{}
	}
	if err != nil {
		log.Println(err)
		return nil
	}
	if !timeStr.Valid {
		log.Println("null time in LastRun call results")
		return nil
	}
	return &timeStr.Time
}

type LanguageCount struct {
	Language string
	Count    int
	Users    int
}

func PopularLanguages(db DBReader) []LanguageCount {
	langs := []LanguageCount{}
	err := db.Select(&langs, queryPopularLanguages)
	if err != nil {
		log.Println(err)
		return nil
	}
	return langs
}

type DevCount struct {
	Login, AvatarUrl, Followers, PublicRepos string
	Name                                     *string
	Stars, Forks                             int
}

func PopularDevs(db DBReader) []DevCount {
	devs := []DevCount{}
	err := db.Select(&devs, queryPopularDevs)
	if err != nil {
		log.Println(err)
		return nil
	}
	return devs
}

func PopularOrgs(db DBReader) []DevCount {
	devs := []DevCount{}
	err := db.Select(&devs, queryPopularOrgs)
	if err != nil {
		log.Println(err)
		return nil
	}
	return devs
}

type LanguageResult struct {
	Owner string
	Repos []stldevs.Repository
	Count int
}

func Language(db DBReader, name string, page int) ([]*LanguageResult, int) {
	repos := []struct {
		stldevs.Repository
		Count int
		Rownum int
	}{}
	err := db.Select(&repos, queryLanguage, name)
	if err != nil {
		log.Println(err)
		return nil, 0
	}
	results := []*LanguageResult{}
	var cursor *LanguageResult
	for _, repo := range repos {
		if cursor == nil || cursor.Owner != *repo.Owner {
			cursor = &LanguageResult{Owner: *repo.Owner, Repos: []stldevs.Repository{repo.Repository}, Count: repo.Count}
			results = append(results, cursor)
		} else {
			cursor.Repos = append(cursor.Repos, repo.Repository)
		}
	}

	var total int
	if err = db.Get(&total, countLanguageUsers, name); err != nil {
		log.Println(err)
	}

	return results, total
}

type ProfileData struct {
	User  *github.User
	Repos map[string][]stldevs.Repository
}

func Profile(db DBReader, name string) (*ProfileData, error) {
	user := &github.User{}
	reposByLang := map[string][]stldevs.Repository{}
	profile := &ProfileData{user, reposByLang}
	err := db.Get(profile.User, queryProfileForUser, name)
	if err != nil {
		log.Println("Error querying profile", name)
		return nil, err
	}

	repos := []stldevs.Repository{}
	err = db.Select(&repos, queryRepoForUser, name)
	if err != nil {
		log.Println("Error querying repo for user", name)
		return nil, err
	}

	for _, repo := range repos {
		lang := *repo.Language
		if _, ok := reposByLang[lang]; !ok {
			reposByLang[lang] = []stldevs.Repository{repo}
			continue
		}
		reposByLang[lang] = append(reposByLang[lang], repo)
	}

	return profile, nil
}

func Search(db DBReader, term, kind string) interface{} {
	query := "%" + term + "%"
	if kind == "users" {
		users := []stldevs.User{}
		if err := db.Select(&users, querySearchUsers, query); err != nil {
			log.Println(err)
			return nil
		}
		return users
	} else if kind == "repos" {
		repos := []stldevs.Repository{}
		if err := db.Select(&repos, querySearchRepos, query); err != nil {
			log.Println(err)
			return nil
		}
		return repos
	}
	log.Println("Unknown search kind", kind)
	return nil
}
