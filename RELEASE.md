# Plugin broker release process

Che plugin broker releases are built automatically from webhooks on the `release` branch and uses the version specified in `VERSION` to tag the built images. To create a new release:

1. Create a `major.minor.x` branch if necessary (e.g. `v3.2.x`)
2. Update `VERSION` file to refer to the new version to be released, with bugfix version number included (e.g. `v3.2.0`)
    - Note the convention for the Che plugin broker is to prefix release versions with `v`, i.e. `v3.2.0` instead of `3.2.0`.
3. Push branch to main repo
4. Reset the `release` branch to the head of your release branch and push it to Github to trigger CI.
5. For non-bugfix releases, bump the major/minor version in VERSION on the master branch

For bugfix releases, the `major.minor.x` branch should be reused, with necessary commits cherry-picked into it.

## Automation
There is a script to automate the release process, named `make-release.sh`:
```
./make-release.sh --repo git@github.com:eclipse/che-plugin-registry --version 3.2.1 --trigger-release
```

See `./make-release.sh --help` for usage.
