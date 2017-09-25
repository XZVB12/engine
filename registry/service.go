package registry

import (
	"net/http"
	"strings"
	"sync"

	"golang.org/x/net/context"

	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/maliceio/engine/api/types"
	registrytypes "github.com/maliceio/engine/api/types/registry"
	"github.com/sirupsen/logrus"
)

const (
	// DefaultSearchLimit is the default value for maximum number of returned search results.
	DefaultSearchLimit = 25
)

// Service is the interface defining what a registry service should implement.
type Service interface {
	LookupPlugin(plugin string) (plugin string, err error)
	Search(ctx context.Context, term string, limit int, authConfig *types.AuthConfig, userAgent string, headers map[string][]string) (*registrytypes.SearchResults, error)
	ServiceConfig() *registrytypes.ServiceConfig
}

// DefaultService is a registry service. It tracks configuration data such as a list
// of mirrors.
type DefaultService struct {
	config *serviceConfig
	mu     sync.Mutex
}

// NewService returns a new instance of DefaultService ready to be
// installed into an engine.
func NewService(options ServiceOptions) *DefaultService {
	return &DefaultService{
		config: newServiceConfig(options),
	}
}

// ServiceConfig returns the public registry service configuration.
func (s *DefaultService) ServiceConfig() *registrytypes.ServiceConfig {
	s.mu.Lock()
	defer s.mu.Unlock()

	servConfig := registrytypes.ServiceConfig{
		AllowNondistributableArtifactsCIDRs:     make([]*(registrytypes.NetIPNet), 0),
		AllowNondistributableArtifactsHostnames: make([]string, 0),
		InsecureRegistryCIDRs:                   make([]*(registrytypes.NetIPNet), 0),
		IndexConfigs:                            make(map[string]*(registrytypes.IndexInfo)),
		Mirrors:                                 make([]string, 0),
	}

	// construct a new ServiceConfig which will not retrieve s.Config directly,
	// and look up items in s.config with mu locked
	servConfig.AllowNondistributableArtifactsCIDRs = append(servConfig.AllowNondistributableArtifactsCIDRs, s.config.ServiceConfig.AllowNondistributableArtifactsCIDRs...)
	servConfig.AllowNondistributableArtifactsHostnames = append(servConfig.AllowNondistributableArtifactsHostnames, s.config.ServiceConfig.AllowNondistributableArtifactsHostnames...)
	servConfig.InsecureRegistryCIDRs = append(servConfig.InsecureRegistryCIDRs, s.config.ServiceConfig.InsecureRegistryCIDRs...)

	for key, value := range s.config.ServiceConfig.IndexConfigs {
		servConfig.IndexConfigs[key] = value
	}

	servConfig.Mirrors = append(servConfig.Mirrors, s.config.ServiceConfig.Mirrors...)

	return &servConfig
}

// LoadMirrors loads registry mirrors for Service
func (s *DefaultService) LoadMirrors(mirrors []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.config.LoadMirrors(mirrors)
}

// splitReposSearchTerm breaks a search term into an index name and remote name
func splitReposSearchTerm(reposName string) (string, string) {
	nameParts := strings.SplitN(reposName, "/", 2)
	var indexName, remoteName string
	if len(nameParts) == 1 || (!strings.Contains(nameParts[0], ".") &&
		!strings.Contains(nameParts[0], ":") && nameParts[0] != "localhost") {
		// This is a Docker Index repos (ex: samalba/hipache or ubuntu)
		// 'docker.io'
		indexName = IndexName
		remoteName = reposName
	} else {
		indexName = nameParts[0]
		remoteName = nameParts[1]
	}
	return indexName, remoteName
}

// Search queries the public registry for images matching the specified
// search terms, and returns the results.
func (s *DefaultService) Search(ctx context.Context, term string, limit int, authConfig *types.AuthConfig, userAgent string, headers map[string][]string) (*registrytypes.SearchResults, error) {
	// TODO Use ctx when searching for repositories
	if err := validateNoScheme(term); err != nil {
		return nil, err
	}

	indexName, remoteName := splitReposSearchTerm(term)

	// Search is a long-running operation, just lock s.config to avoid block others.
	s.mu.Lock()
	index, err := newIndexInfo(s.config, indexName)
	s.mu.Unlock()

	if err != nil {
		return nil, err
	}

	// *TODO: Search multiple indexes.
	endpoint, err := NewV1Endpoint(index, userAgent, http.Header(headers))
	if err != nil {
		return nil, err
	}

	var client *http.Client
	if authConfig != nil && authConfig.IdentityToken != "" && authConfig.Username != "" {
		creds := NewStaticCredentialStore(authConfig)
		scopes := []auth.Scope{
			auth.RegistryScope{
				Name:    "catalog",
				Actions: []string{"search"},
			},
		}

		modifiers := DockerHeaders(userAgent, nil)
		v2Client, foundV2, err := v2AuthHTTPClient(endpoint.URL, endpoint.client.Transport, modifiers, creds, scopes)
		if err != nil {
			if fErr, ok := err.(fallbackError); ok {
				logrus.Errorf("Cannot use identity token for search, v2 auth not supported: %v", fErr.err)
			} else {
				return nil, err
			}
		} else if foundV2 {
			// Copy non transport http client features
			v2Client.Timeout = endpoint.client.Timeout
			v2Client.CheckRedirect = endpoint.client.CheckRedirect
			v2Client.Jar = endpoint.client.Jar

			logrus.Debugf("using v2 client for search to %s", endpoint.URL)
			client = v2Client
		}
	}

	if client == nil {
		client = endpoint.client
		if err := authorizeClient(client, authConfig, endpoint); err != nil {
			return nil, err
		}
	}

	r := newSession(client, authConfig, endpoint)

	if index.Official {
		localName := remoteName
		if strings.HasPrefix(localName, "library/") {
			// If pull "library/foo", it's stored locally under "foo"
			localName = strings.SplitN(localName, "/", 2)[1]
		}

		return r.SearchRepositories(localName, limit)
	}
	return r.SearchRepositories(remoteName, limit)
}

// ResolveRepository splits a repository name into its components
// and configuration of the associated registry.
func (s *DefaultService) ResolveRepository(name reference.Named) (*RepositoryInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return newRepositoryInfo(s.config, name)
}
