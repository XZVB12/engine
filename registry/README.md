registry
========

This is malice's ability to reach out to [maliceio/registry](https://github.com/maliceio/registry) and pull down the most up-to-date copy of registered malice plugins, as well as pulling some metrics:


Design
------

```go
type SearchResult struct {
    // StarCount indicates the number of stars this repository has
    StarCount int `json:"star_count"`
    // IsOfficial is true if the result is from an official repository.
    IsOfficial bool `json:"is_official"`
    // Name is the name of the repository
    Name string `json:"name"`
    // IsAutomated indicates whether the result is automated
    IsAutomated bool `json:"is_automated"`
    // Description is a textual description of the repository
    Description string `json:"description"`
}
```

this will be stored as a `registry.json` in the `.malice/registry` config directory.

This git repo **git pull** will need to authenticated as I will be signing it with my keybase.io PGP key.

I will also include the sha256 somewhere for the json file?

Features
--------


TODO
----

- [ ] add gitlab API code to pull plugin metrics (stars, name, description, etc.)
- [ ] add [pgp](https://godoc.org/golang.org/x/crypto/openpgp) code to verify **maliceio/registry** code