package aggregator

import (
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/go-github/github"
)

func (a *Aggregator) updateUsers(users map[string]struct{}) error {
	existingUsers := []github.User{}
	err := a.db.Select(&existingUsers, `SELECT login FROM agg_user`)
	if err != nil {
		log.Println("Error querying agg_user")
		return err
	}

	// remove users that no longer come up in search
	for _, existing := range existingUsers {
		if _, ok := users[existing]; !ok {
			_, err = a.db.Exec(`DELETE FROM agg_user WHERE login=?`, existing)
			if err != nil {
				log.Println("Error while deleting moved user:", existing)
			}
			_, err = a.db.Exec(`DELETE FROM agg_repo WHERE owner=?`, existing)
			if err != nil {
				log.Println("Error while deleting moved user's repos", existing)
			}
		}
	}

	for user := range users {
		a.Add(user)
	}
	return nil
}

func (a *Aggregator) updateRepos() error {
	rows, err := a.db.Query("select login from agg_user")
	if err != nil {
		log.Println("error whilst selecting from agg_user")
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var user string
		if err = rows.Scan(&user); err != nil {
			log.Println("Error scanning")
			return err
		}
		a.updateUsersRepos(user)
	}
	return nil
}

func (a *Aggregator) updateUsersRepos(user string) error {
	opts := &github.RepositoryListOptions{Type: "owner", Sort: "updated", Direction: "desc", ListOptions: github.ListOptions{PerPage: 100}}
	for {
		result, resp, err := a.client.Repositories.List(user, opts)
		if err != nil {
			log.Println("error while listing repositories")
			return err
		}
		checkRespAndWait(resp)
		for _, repo := range result {
			var pushedAt *time.Time
			if repo.PushedAt != nil {
				pushedAt = &repo.PushedAt.Time
			}
			_, err = a.db.Exec(`REPLACE INTO agg_repo VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
				repo.Owner.Login, repo.Name, repo.Description, repo.Language, repo.Homepage,
				repo.ForksCount, repo.NetworkCount, repo.OpenIssuesCount, repo.StargazersCount, repo.SubscribersCount,
				repo.WatchersCount, repo.Size, *repo.Fork, repo.DefaultBranch, repo.MasterBranch, repo.CreatedAt.Time,
				pushedAt, repo.UpdatedAt.Time)
			if err != nil {
				log.Println("Error executing replace into agg_repo")
				return err
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return nil
}

func (a *Aggregator) findStlUsers() (map[string]struct{}, error) {
	searchString := `location:"St. Louis"  location:"STL" location:"St Louis" location:"Saint Louis"`
	opts := &github.SearchOptions{Sort: "followers", Order: "desc", ListOptions: github.ListOptions{Page: 1, PerPage: 100}}
	users := map[string]struct{}{}
	for {
		result, resultResp, err := a.client.Search.Users(searchString, opts)
		if err != nil {
			log.Println("Error Searching users")
			return nil, err
		}
		checkRespAndWait(resultResp)
		for _, user := range result.Users {
			users[*user.Login] = struct{}{}
		}
		if resultResp.NextPage == 0 {
			break
		}

		opts.ListOptions.Page = resultResp.NextPage
	}
	fmt.Printf("Total found: %v\n", len(users))
	return users, nil
}

func (a *Aggregator) Add(user string) error {
	u, resp, err := a.client.Users.Get(user)
	if err != nil {
		log.Println("Failed getting user details for", user, ":", err)
		return err
	}
	checkRespAndWait(resp)
	_, err = a.db.Exec(`REPLACE INTO agg_user VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		u.Login, u.Email, u.Name, u.Location, u.Hireable, u.Blog, u.Bio, u.Followers, u.Following,
		u.PublicRepos, u.PublicGists, u.AvatarURL, u.DiskUsage, u.CreatedAt.Time, u.UpdatedAt.Time)
	return err
}

func (a *Aggregator) insertRunLog() error {
	_, err := a.db.Exec(`INSERT INTO agg_meta VALUES (?)`, time.Now())
	if err != nil {
		log.Println("Error executing insert")
	}
	return err
}

func checkRespAndWait(r *github.Response) {
	if r.Remaining == 0 {
		duration := time.Now().Sub(r.Rate.Reset.Time)
		fmt.Println("I ran out of requests, waiting", duration)
		time.Sleep(duration)
	} else {
		fmt.Println(r.Remaining, "calls remaining until", r.Rate.Reset.String())
	}
}
