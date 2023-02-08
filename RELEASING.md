# Release Process

## Pre-Release

```
$ git fetch
$ git checkout origin/main -b pre-release
$ # ensure that version numbers in tools/release.go are what you want
$ make prepare-release
$ # update test fixtures to reflect new versions
$ make fixtures
$ git commit -a
$ git push -u origin HEAD
$ # create a PR with a link to draft release notes
$ # get the PR reviewed and merged
```

## Release

```
$ # do this after the pre-release PR is merged
$ git fetch
$ git checkout origin/main
$ make release
$ # make sure you don't have any stray tags lying around
$ git push --tags
```

***IMPORTANT***: It is critical you use the same tag that you used in the Pre-Release step!
Failure to do so will leave things in a broken state.

***IMPORTANT***: [There is currently no way to remove an incorrectly tagged version of a Go module](https://github.com/golang/go/issues/34189).
It is critical you make sure the version you push upstream is correct.
[Failure to do so will lead to minor emergencies and tough to work around](https://github.com/open-telemetry/opentelemetry-go/issues/331).

## Post-release

* Verify the examples
```
./verify_examples.sh
```

The script copies examples into a different directory removes any `replace` declarations in `go.mod` and builds them.
This ensures they build with the published release, not the local copy.

* Update the examples maintained outside this repo at https://github.com/GoogleCloudPlatform/golang-samples/tree/master/opentelemetry.

* Update the collector exporter
