package reference

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/distribution/reference"
	digest "github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/require"
)

var (
	saveLoadTestCases = map[string]digest.Digest{}

	marshalledSaveLoadTestCases = []byte(`{"Repositories":{"busybox":{"busybox:latest":"sha256:91e54dfb11794fad694460162bf0cb0a4fa710cfa3f60979c177d920813e267c"},"jess/hollywood":{"jess/hollywood:latest":"sha256:ae7a5519a0a55a2d4ef20ddcbd5d0ca0888a1f7ab806acc8e2a27baf46f529fe"},"registry":{"registry@sha256:367eb40fd0330a7e464777121e39d2f5b3e8e23a1e159342e53ab05c9e4d94e6":"sha256:24126a56805beb9711be5f4590cc2eb55ab8d4a85ebd618eed72bb19fc50631c"},"registry:5000/foobar":{"registry:5000/foobar:HEAD":"sha256:470022b8af682154f57a2163d030eb369549549cba00edc69e1b99b46bb924d6","registry:5000/foobar:alternate":"sha256:ae300ebc4a4f00693702cfb0a5e0b7bc527b353828dc86ad09fb95c8a681b793","registry:5000/foobar:latest":"sha256:6153498b9ac00968d71b66cca4eac37e990b5f9eb50c26877eb8799c8847451b","registry:5000/foobar:master":"sha256:6c9917af4c4e05001b346421959d7ea81b6dc9d25718466a37a6add865dfd7fc"}}}`)
)

func TestLoad(t *testing.T) {
	jsonFile, err := ioutil.TempFile("", "tag-store-test")
	if err != nil {
		t.Fatalf("error creating temp file: %v", err)
	}
	defer os.RemoveAll(jsonFile.Name())

	// Write canned json to the temp file
	_, err = jsonFile.Write(marshalledSaveLoadTestCases)
	if err != nil {
		t.Fatalf("error writing to temp file: %v", err)
	}
	jsonFile.Close()

	store, err := NewReferenceStore(jsonFile.Name())
	if err != nil {
		t.Fatalf("error creating tag store: %v", err)
	}

	for refStr, expectedID := range saveLoadTestCases {
		ref, err := reference.ParseNormalizedNamed(refStr)
		if err != nil {
			t.Fatalf("failed to parse reference: %v", err)
		}
		id, err := store.Get(ref)
		if err != nil {
			t.Fatalf("could not find reference %s: %v", refStr, err)
		}
		if id != expectedID {
			t.Fatalf("expected %s - got %s", expectedID, id)
		}
	}
}

func TestSave(t *testing.T) {
	jsonFile, err := ioutil.TempFile("", "tag-store-test")
	require.NoError(t, err)

	_, err = jsonFile.Write([]byte(`{}`))
	require.NoError(t, err)
	jsonFile.Close()
	defer os.RemoveAll(jsonFile.Name())

	store, err := NewReferenceStore(jsonFile.Name())
	if err != nil {
		t.Fatalf("error creating tag store: %v", err)
	}

	for refStr, id := range saveLoadTestCases {
		ref, err := reference.ParseNormalizedNamed(refStr)
		if err != nil {
			t.Fatalf("failed to parse reference: %v", err)
		}
		if canonical, ok := ref.(reference.Canonical); ok {
			err = store.AddDigest(canonical, id, false)
			if err != nil {
				t.Fatalf("could not add digest reference %s: %v", refStr, err)
			}
		} else {
			err = store.AddTag(ref, id, false)
			if err != nil {
				t.Fatalf("could not add reference %s: %v", refStr, err)
			}
		}
	}

	jsonBytes, err := ioutil.ReadFile(jsonFile.Name())
	if err != nil {
		t.Fatalf("could not read json file: %v", err)
	}

	if !bytes.Equal(jsonBytes, marshalledSaveLoadTestCases) {
		t.Fatalf("save output did not match expectations\nexpected:\n%s\ngot:\n%s", marshalledSaveLoadTestCases, jsonBytes)
	}
}

func TestAddDeleteGet(t *testing.T) {
	jsonFile, err := ioutil.TempFile("", "tag-store-test")
	if err != nil {
		t.Fatalf("error creating temp file: %v", err)
	}
	_, err = jsonFile.Write([]byte(`{}`))
	jsonFile.Close()
	defer os.RemoveAll(jsonFile.Name())
}
