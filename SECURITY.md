# Security policy

mailpitter is a local development tool. It runs one Mailpit container in
Docker with no SMTP authentication, publishes its SMTP and UI ports on the
host's loopback address, and registers the UI with caddy_shack so it is
reachable at `https://mailpit.test`. On native Linux Docker Engine it also
publishes SMTP on the docker0 bridge gateway so other containers can reach
it through `host-gateway`. The README's
[Reaching SMTP from other containers](README.md#reaching-smtp-from-other-containers)
describes that in full. Nothing binds `0.0.0.0`, mailpitter never runs
`sudo` or edits the firewall, and its state file and lock live under the
user's own configuration directory.

Bugs that weaken that boundary are security bugs: SMTP or the UI reachable
from beyond the host and its own containers; the state file or lock being
usable by another local user to change what mailpitter does; mailpitter
running anything with elevated privileges; or the `docker` and
`caddy_shack` commands it runs taking arguments from input it does not
control. Behaviour the README already documents as a known limit (for
example, that any container on the host can send mail to Mailpit) is not a
vulnerability, though a better fix is always welcome.

The release gate (the check that a tag is on `main` and signed) relies on
nobody but the maintainer having write access to the repository, since a
collaborator with write access could alter the workflow. The `v*` tag
ruleset blocks moving or deleting release tags.

## Supported versions

Only the newest release on the
[Releases page](https://github.com/bawdo/mailpitter/releases) receives
fixes. There are no maintenance branches.

## Reporting a vulnerability

Please do not open a public issue for a security problem.

Use GitHub's private reporting form:
<https://github.com/bawdo/mailpitter/security/advisories/new>. It goes
straight to the maintainer and nobody else can see it.

Include what you can of: the output of `mailpitter version` and
`mailpitter doctor`, the output of `caddy_shack version`, the Docker engine
you run (Docker Desktop, OrbStack, Colima or native Linux) and its version,
the steps involved, what you expected, and what happened. A proof of
concept helps but is not required.

You will get an acknowledgement within seven days. A fix, or an explanation
of why the report is not being treated as a vulnerability, follows as a
published advisory and a release. Credit is given in the advisory unless
you ask otherwise.

## Verifying a release

Every release archive carries a build provenance attestation from the
release workflow, and a `checksums.txt` lists the archive digests. To
confirm a download came from this repository's CI and has not been
altered, run both against the archive you downloaded:

```bash
gh attestation verify mailpitter_<version>_<os>_<arch>.tar.gz --repo bawdo/mailpitter \
  --signer-workflow bawdo/mailpitter/.github/workflows/release.yml
sha256sum -c checksums.txt --ignore-missing
```

On macOS, use `shasum -a 256 -c checksums.txt --ignore-missing` for the
second command.
