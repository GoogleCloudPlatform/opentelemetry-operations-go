# Release Process

## Pre-Release

Update go.mod for submodules to depend on the new release which will happen in the next step.

1. Run the pre-release script. It creates a branch `pre_release_<new tag>` that will contain all release changes.

    ```
    ./pre_release.sh -t <new tag>
    ```

2. Verify the changes.

    ```
    git diff main
    ```

    This should have changed the version for all modules to be `<new tag>`.

3. TODO: Consider adding [Changelog](./CHANGELOG.md) and update this for each release.

4. Push the changes to upstream and create a Pull Request on GitHub.
    Be sure to include the curated changes from the [Changelog](./CHANGELOG.md) in the description.


## Tag

Once the Pull Request with all the version changes has been approved and merged it is time to tag the merged commit.

***IMPORTANT***: It is critical you use the same tag that you used in the Pre-Release step!
Failure to do so will leave things in a broken state.

***IMPORTANT***: [There is currently no way to remove an incorrectly tagged version of a Go module](https://github.com/golang/go/issues/34189).
It is critical you make sure the version you push upstream is correct.
[Failure to do so will lead to minor emergencies and tough to work around](https://github.com/open-telemetry/opentelemetry-go/issues/331).

1. Run the tag.sh script using the `<commit-hash>` of the commit on the main branch for the merged Pull Request.

    ```
    ./tag.sh <new tag> <commit-hash>
    ```

2. Push tags to the upstream remote (not your fork: `github.com/GoogleCloudPlatform/opentelemetry-operations-go.git`).

    ```
    git push upstream <new tag>
    ```

## Release

Finally create a Release for the new `<new tag>` on GitHub.

Make sure all relevant changes for this release are included in the release notes and are in language that non-contributors to the project can understand. The `tag.sh` script generates commit logs since last release which can be used to derive the release notes from. These are generated using:

```
git --no-pager log --pretty=oneline "<last tag>..HEAD"
```

## Verify Examples

After releasing verify that examples build outside of the repository.

```
./verify_examples.sh
```

The script copies examples into a different directory removes any `replace` declarations in `go.mod` and builds them.
This ensures they build with the published release, not the local copy.

Also update the examples maintained outside this repo at https://github.com/GoogleCloudPlatform/golang-samples/tree/master/opentelemetry.
