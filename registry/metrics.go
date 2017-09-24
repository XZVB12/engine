package registry

import "github.com/google/go-github/github"

// RepoMetrics the repo metrics struct
type RepoMetrics struct {
	Stars int
}

// GetMetricsForRepo reads plugin metrics from Github API
func GetMetricsForRepo(repo string) (RepoMetrics, error) {
	rm := RepoMetrics{}

	client := github.NewClient(nil)

	// list public repositories for org "github"
	opt := &github.RepositoryListByOrgOptions{Type: "public"}
	repos, _, err := client.Repositories.ListByOrg(ctx, "github", opt)
	if err != nil {
		return rm, err
	}
	rm.Stars = 1

	return rm, nil
}
