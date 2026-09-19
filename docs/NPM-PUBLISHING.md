# Publishing the npm installer

The `npm/cli` package is a thin installer for the official GitHub Release. It contains a Node launcher and an installer only; it downloads the platform-specific release archive, checks the named SHA-256 entry, accepts only an archive containing the expected executable, and then starts that executable.

The public package name is `harnessforge`, so users install it with:

```sh
npm install -g harnessforge
```

## Initial package setup

1. Create or sign in to the npm account that will own the unscoped `harnessforge` package. Enable two-factor authentication for package publishing.
2. From `npm/cli`, set the package version to the already-published GitHub Release version and publish it with public access. Review `npm pack --dry-run --ignore-scripts` before publishing.
3. In the package settings on npmjs.com, add `joicepassos/harness-forge` as a GitHub Actions trusted publisher. Set the workflow filename to `release.yml`; no npm token is needed after trusted publishing is configured.
4. In the GitHub repository, create the Actions variable `NPM_PUBLISH_ENABLED` with value `true`. Future `v1.*` releases then publish the matching npm version after the GitHub Release has passed tests, binary scans, attestations, and the protected release approval.

The initial publish is intentionally manual because npm requires a package owner before its trusted-publisher setting can be created. Future publication uses GitHub OIDC (`id-token: write`) and does not require an NPM access token in repository secrets. npm documents the trusted-publisher setup at <https://docs.npmjs.com/trusted-publishers/>.

## Initial publication commands

Run these from a reviewed checkout after authenticating with `npm login`:

```sh
cd npm/cli
npm test
npm pack --dry-run --ignore-scripts
npm version 1.0.2 --no-git-tag-version
npm publish --access public
```

Do not publish a package version unless the GitHub Release with the same version already exists. The release workflow sets the npm package version from the protected Git tag in its temporary workspace.

## Testing without publishing

`npm test` validates platform mapping, manifest selection, and HTTPS-only download policy. `npm pack --dry-run --ignore-scripts` shows the exact files the registry would receive. Do not use `npm install` as a package test from the repository checkout unless the package version corresponds to an existing GitHub Release: the postinstall hook deliberately downloads that matching release archive.
