git
===

NOTE
----

Requires [libgit2](https://github.com/libgit2/libgit2) to run we need to install it or figure a way to statically build it into [git2go](https://github.com/libgit2/git2go)

Here is the **alpine** package: https://pkgs.alpinelinux.org/package/v3.6/main/aarch64/libgit2

Example
-------

```go
package main

import (
	"github.com/maliceio/engine/registry/git"
	"github.com/sirupsen/logrus"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

func main() {
	err := git.CloneRegistry()
	if err != nil {
		logrus.Fatal(err)
	}
	err = git.Pull()
	if err != nil {
		logrus.Fatal(err)
	}
}
```

```sh
DEBU[0000] malice registry has been cloned into: /Users/user/.malice/registry
DEBU[0000] good signature from: blacktop <blacktop@users.noreply.github.com>
DEBU[0000] malice registry has been updated
```