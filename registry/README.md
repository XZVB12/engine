registry
========

`registry` contains code to reach out to [maliceio/registry](https://github.com/maliceio/registry) and pull down the most up-to-date copy of registered malice plugins, as well as pulling some metrics:

```go
type RepoMetric struct {
    Name        string `json:"name,omitempty"`
    Description string `json:"description,omitempty"`
    Stars       int    `json:"stars,omitempty"`
    Forks       int    `json:"forks,omitempty"`
    Watchers    int    `json:"watchers,omitempty"`
}
```

Design
------

The `malice/registry` will be stored in the `~/.malice/registry` directory

This git repo [maliceio/registry](https://github.com/maliceio/registry) will need to verified similar to the way `git verify-commit` works.  I will be signing every commit with my [keybase.io](https://keybase.io/blacktop) PGP key.

Features
--------

git commands

- `git clone` [maliceio/registry](https://github.com/maliceio/registry)
- `git verify-commit` of **HEAD**
- `git pull` to update registry

plugin metrics

- uses Github API to pull metrics on plugins in registry to help users find the plugins they are looking for

TODO
----

- [x] add gitlab API code to pull plugin metrics (stars, name, description, etc.)
- [x] add [pgp](https://godoc.org/golang.org/x/crypto/openpgp) code to verify **maliceio/registry** code

Notes
-----

To pull my **pgp** public key you can

```sh
$ curl https://keybase.io/blacktop/pgp_keys.asc | gpg --import
```

It will also be embedded in the maliced binary.