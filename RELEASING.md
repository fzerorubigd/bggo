# Releasing

Releases are cut by [release-please](https://github.com/googleapis/release-please) from [Conventional Commits](https://www.conventionalcommits.org/). Versions are git tags (`vMAJOR.MINOR.PATCH`).

## How it works

1. Commits that land on `main` use conventional messages (`feat:`, `fix:`, `docs:`, `chore:`, …). Pull requests are merged with a merge commit, so it is each commit's own message that counts; the merge commit itself and commits without a prefix (such as dependabot's "Bump …") do not change the version. Before 1.0, `feat:` bumps the minor version, `fix:` the patch, and a breaking change the minor (`bump-minor-pre-major`).
2. On every push to `main`, the `release-please` workflow keeps a release PR open with the next version and its `CHANGELOG.md` entry. Versions are computed from the one in `.release-please-manifest.json`, which starts at 0.2.1, the last tag cut by hand.
3. Merging the release PR creates the tag and the GitHub release. The same workflow run then calls `release.yml`, which builds the `bgg-mcp` binaries from the tag and attaches them to that release.

## The release PR needs one manual step

The release PR is opened with the built-in `GITHUB_TOKEN`, and GitHub does not start workflows for events made with that token. So CI does not run on the release PR on its own. Close the release PR and reopen it once: reopening it as a person starts CI. Then review and merge as usual.

For the same reason, the tag that release-please pushes does not start `release.yml` through its tag trigger. That is why `release-please.yml` calls `release.yml` directly.

## Releasing by hand

Pushing a `v*` tag still runs `release.yml` on its own: it creates the release with generated notes and attaches the binaries. Bump `.release-please-manifest.json` to the same version in a commit, or release-please's next version will be computed from the old one.
