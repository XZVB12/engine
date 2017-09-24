git
===

NOTE
----

Requires [libgit2](https://github.com/libgit2/libgit2) to run we need to install it or figure a way to statically build it into [git2go](https://github.com/libgit2/git2go)

Here is the **alpine** package: https://pkgs.alpinelinux.org/package/v3.6/main/aarch64/libgit2

Example
-------

Clone `maliceio/registry`

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
	err = git.PullRegistry()
	if err != nil {
		logrus.Fatal(err)
	}
}
```

```sh
DEBU[0000] malice registry has been cloned into: ~/.malice/registry
DEBU[0000] good signature from: blacktop <blacktop@users.noreply.github.com>
DEBU[0000] malice registry has been updated
```

Get malice plugin **metrics**

```go
package main

import "github.com/maliceio/engine/registry/git"

func main() {
	git.GetMetricsForRepo("")
}
```

```sh
plugin: fileinfo, stars: 9
plugin: yara, stars: 9
plugin: exe, stars: 7
plugin: nsrl, stars: 4
plugin: virustotal, stars: 4
plugin: office, stars: 4
plugin: pdf, stars: 4
plugin: windows-defender, stars: 3
plugin: floss, stars: 3
plugin: malice-alpine, stars: 2
plugin: fprot, stars: 2
plugin: bro, stars: 2
plugin: javascript, stars: 2
plugin: go-plugin-utils, stars: 2
plugin: totalhash, stars: 2
plugin: fsecure, stars: 1
plugin: team-cymru, stars: 1
plugin: threat-expert, stars: 1
plugin: sophos, stars: 1
plugin: anubis, stars: 1
plugin: archive, stars: 1
plugin: comodo, stars: 1
plugin: clamav, stars: 1
plugin: bitdefender, stars: 1
plugin: avg, stars: 1
plugin: avast, stars: 1
plugin: shadow-server, stars: 1
plugin: avira, stars: 1
plugin: zoner, stars: 1
plugin: escan, stars: 1
```