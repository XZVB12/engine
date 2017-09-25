package reference

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/docker/cli/cli/config"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	// ErrDoesNotExist is returned if a reference is not found in the
	// store.
	ErrDoesNotExist notFoundError = "reference does not exist"

	referenceFilePath string
)

// Store provides the set of methods which can operate on a reference store.
type Store interface {
	Repos(id digest.Digest) []reference.Named
	ReposByName(ref reference.Named)
	// AddRepo(ref reference.Canonical, id digest.Digest, force bool) error
	Delete(ref reference.Named) (bool, error)
	Get(ref reference.Named) (digest.Digest, error)
}

// repository
type repository struct {
}

type store struct {
	mu sync.RWMutex
	// jsonPath is the path to the file where the serialized repo data is
	// stored.
	jsonPath string
	// Repositories is a map of repositories, indexed by name.
	Repositories map[string]repository
	// referencesByIDCache is a cache of references indexed by ID, to speed
	// up References.
	referencesByIDCache map[digest.Digest]map[string]reference.Named
}

func init() {
	if referenceFilePath == "" {
		referenceFilePath = filepath.Join(config.Dir(), "repositories.json")
	}
	logrus.Debugf("malice reference.json path is %s", referenceFilePath)
}

// NewReferenceStore creates a new reference store, tied to a file path where
// the set of references are serialized in JSON format.
func NewReferenceStore(jsonPath string) (Store, error) {
	abspath, err := filepath.Abs(jsonPath)
	if err != nil {
		return nil, err
	}

	store := &store{
		jsonPath:            abspath,
		Repositories:        make(map[string]repository),
		referencesByIDCache: make(map[digest.Digest]map[string]reference.Named),
	}
	// Load the json file if it exists, otherwise create it.
	if err := store.reload(); os.IsNotExist(err) {
		if err := store.save(); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	return store, nil
}

func (store *store) addReference(ref reference.Named, id digest.Digest, force bool) error {

	store.mu.Lock()
	defer store.mu.Unlock()

	repository, exists := store.Repositories[refName]
	if !exists || repository == nil {
		repository = make(map[string]digest.Digest)
		store.Repositories[refName] = repository
	}

	oldID, exists := repository[refStr]

	if exists {
		// force only works for tags
		if digested, isDigest := ref.(reference.Canonical); isDigest {
			return errors.WithStack(conflictingTagError("Cannot overwrite digest " + digested.Digest().String()))
		}

		if store.referencesByIDCache[oldID] != nil {
			delete(store.referencesByIDCache[oldID], refStr)
			if len(store.referencesByIDCache[oldID]) == 0 {
				delete(store.referencesByIDCache, oldID)
			}
		}
	}

	repository[refStr] = id
	if store.referencesByIDCache[id] == nil {
		store.referencesByIDCache[id] = make(map[string]reference.Named)
	}
	store.referencesByIDCache[id][refStr] = ref

	return store.save()
}

// Delete deletes a reference from the store. It returns true if a deletion
// happened, or false otherwise.
func (store *store) Delete(ref reference.Named) (bool, error) {
	ref, err := favorDigest(ref)
	if err != nil {
		return false, err
	}

	ref = reference.TagNameOnly(ref)

	refName := reference.FamiliarName(ref)
	refStr := reference.FamiliarString(ref)

	store.mu.Lock()
	defer store.mu.Unlock()

	repository, exists := store.Repositories[refName]
	if !exists {
		return false, ErrDoesNotExist
	}

	if id, exists := repository[refStr]; exists {
		delete(repository, refStr)
		if len(repository) == 0 {
			delete(store.Repositories, refName)
		}
		if store.referencesByIDCache[id] != nil {
			delete(store.referencesByIDCache[id], refStr)
			if len(store.referencesByIDCache[id]) == 0 {
				delete(store.referencesByIDCache, id)
			}
		}
		return true, store.save()
	}

	return false, ErrDoesNotExist
}

// Get retrieves an item from the store by reference
func (store *store) Get(ref reference.Named) (digest.Digest, error) {
	if canonical, ok := ref.(reference.Canonical); ok {
		// If reference contains both tag and digest, only
		// lookup by digest as it takes precedence over
		// tag, until tag/digest combos are stored.
		if _, ok := ref.(reference.Tagged); ok {
			var err error
			ref, err = reference.WithDigest(reference.TrimNamed(canonical), canonical.Digest())
			if err != nil {
				return "", err
			}
		}
	} else {
		ref = reference.TagNameOnly(ref)
	}

	refName := reference.FamiliarName(ref)
	refStr := reference.FamiliarString(ref)

	store.mu.RLock()
	defer store.mu.RUnlock()

	repository, exists := store.Repositories[refName]
	if !exists || repository == nil {
		return "", ErrDoesNotExist
	}

	id, exists := repository[refStr]
	if !exists {
		return "", ErrDoesNotExist
	}

	return id, nil
}

// References returns a slice of references to the given ID. The slice
// will be nil if there are no references to this ID.
func (store *store) Repos(id digest.Digest) []reference.Named {
	store.mu.RLock()
	defer store.mu.RUnlock()

	// Convert the internal map to an array for two reasons:
	// 1) We must not return a mutable
	// 2) It would be ugly to expose the extraneous map keys to callers.

	var references []reference.Named
	for _, ref := range store.referencesByIDCache[id] {
		references = append(references, ref)
	}

	sort.Sort(lexicalRefs(references))

	return references
}

func (store *store) ReposByName(ref reference.Named) []Association {
	refName := reference.FamiliarName(ref)

	store.mu.RLock()
	defer store.mu.RUnlock()

	repository, exists := store.Repositories[refName]
	if !exists {
		return nil
	}

	var associations []Association
	for refStr, refID := range repository {
		ref, err := reference.ParseNormalizedNamed(refStr)
		if err != nil {
			// Should never happen
			return nil
		}
		associations = append(associations,
			Association{
				Ref: ref,
				ID:  refID,
			})
	}

	sort.Sort(lexicalAssociations(associations))

	return associations
}

func (store *store) save() error {
	// Store the json
	jsonData, err := json.Marshal(store)
	if err != nil {
		return err
	}
	return ioutils.AtomicWriteFile(store.jsonPath, jsonData, 0600)
}

func (store *store) reload() error {
	f, err := os.Open(store.jsonPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&store); err != nil {
		return err
	}

	for _, repository := range store.Repositories {
		for refStr, refID := range repository {
			ref, err := reference.ParseNormalizedNamed(refStr)
			if err != nil {
				// Should never happen
				continue
			}
			if store.referencesByIDCache[refID] == nil {
				store.referencesByIDCache[refID] = make(map[string]reference.Named)
			}
			store.referencesByIDCache[refID][refStr] = ref
		}
	}

	return nil
}
