package git

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/net/context"
)

const timeout = 3

// RepoMetric the repo metrics struct
type RepoMetric struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Stars       int    `json:"stars,omitempty"`
	Forks       int    `json:"forks,omitempty"`
	Watchers    int    `json:"watchers,omitempty"`
}

// GetMetricsForOrg reads org metrics from Github API
func GetMetricsForOrg(org string) (RepoMetric, error) {
	rm := RepoMetric{}
	rms := []RepoMetric{}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	client := github.NewClient(nil)

	// list public repositories for org "github"
	opt := &github.RepositoryListByOrgOptions{Type: "public"}
	repos, _, err := client.Repositories.ListByOrg(ctx, org, opt)
	if err != nil {
		return rm, err
	}

	for _, repo := range repos {
		rm := RepoMetric{}
		rm.Name = repo.GetName()
		rm.Description = repo.GetDescription()
		rm.Stars = repo.GetStargazersCount()
		rm.Forks = repo.GetForksCount()
		rm.Watchers = repo.GetWatchersCount()
		// add to slice
		rms = append(rms, rm)
	}

	sort.Slice(rms, func(i, j int) bool {
		return rms[i].Stars > rms[j].Stars
	})

	for _, r := range rms {
		fmt.Printf("plugin: %s, stars: %d\n", r.Name, r.Stars)
	}

	return rm, nil
}

// GetMetricsForUser reads user metrics from Github API
func GetMetricsForUser(user string) (RepoMetric, error) {
	rm := RepoMetric{}
	rms := []RepoMetric{}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	client := github.NewClient(nil)

	// list public repositories for org "github"
	// opt := &github.RepositoryListByOrgOptions{Type: "public"}
	repos, _, err := client.Repositories.List(ctx, user, &github.RepositoryListOptions{})
	if err != nil {
		return rm, err
	}

	for _, repo := range repos {
		rm := RepoMetric{}
		rm.Name = repo.GetName()
		rm.Description = repo.GetDescription()
		rm.Stars = repo.GetStargazersCount()
		rm.Forks = repo.GetForksCount()
		rm.Watchers = repo.GetWatchersCount()
		// add to slice
		rms = append(rms, rm)
	}

	sort.Slice(rms, func(i, j int) bool {
		return rms[i].Stars > rms[j].Stars
	})

	for _, r := range rms {
		fmt.Printf("plugin: %s, stars: %d\n", r.Name, r.Stars)
	}

	return rm, nil
}
