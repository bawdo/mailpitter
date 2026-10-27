# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-10-27

### Added

- `install` (idempotent, with `--ephemeral`), `start`, `stop`, `restart`
  and `upgrade` of the one shared Mailpit container, keeping stored mail
  in the `mailpitter_data` volume across upgrades and recreations.
- Registration of the Mailpit UI with caddy_shack at `https://mailpit.test`,
  kept in step by `start` and `stop`.
- `status` and `doctor`, each with `--json`, and `doctor --fix` for the
  problems it can repair.
- `register` and `deregister` for consumer projects, printing a `.env` and
  `docker-compose.yml` snippet with `--mail-vars django|laravel`, and
  `list` to show them.
- `open` to launch the UI and `reset` to wipe the container and data
  volume while keeping registered consumers.
- A documented exit-code contract (codes 0 to 9) for scripting.
- SMTP published on the docker0 bridge gateway as well as loopback on
  native Linux Docker Engine, so `host-gateway` works from other
  containers, with the ufw rule printed when the firewall would block it.
- `version`, which prints the build identity.

[Unreleased]: https://github.com/bawdo/mailpitter/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/bawdo/mailpitter/releases/tag/v0.1.0
