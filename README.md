# go-helium-sync

Sync [Helium](https://helium.computer) browser bookmarks between Linux machines, using a private git repo as transport.

Helium (Chromium-based) stores bookmarks in `~/.config/net.imput.helium/Default/Bookmarks` as JSON. This tool reads that file, merges changes by each bookmark's stable `guid`, and writes it back atomically while Helium is closed.

```mermaid
    flowchart LR
        A{Helium closed?} -->|no| X[abort]
        A -->|yes| B[read Bookmarks + backup]
        B --> C[git fetch]
        C --> D[3-way merge by guid<br/>base = last sync]
        D --> E[atomic write]
        E --> F[git commit + push]
```

Inspired by [aadarwal/helium-sync](https://github.com/aadarwal/helium-sync) (macOS, Python).

## Install

Download the `.deb`, `.rpm` or tarball from [Releases](https://github.com/etams20/go-helium-sync/releases), e.g.:

```sh
sudo apt install ./hesync_*_linux_amd64.deb
```

or with Go:

```sh
go install github.com/etaMS20/go-helium-sync/cmd/hesync@latest
```

## Usage

Close Helium first, then once per device:

```sh
hesync setup                # guided: creates/finds a private GitHub repo via gh, or asks for a URL
hesync setup <remote-url>   # or provide an existing private repo
```

The first device seeds the repo, later ones merge into it.
Without [`gh`](https://cli.github.com), create a private repo yourself (GitHub, GitLab, ...) first.

hesync only reads and writes `bookmarks.json` on the `main` branch. Other files (README, LICENSE, ...)
are left alone, so the repo doesn't have to be empty, but `bookmarks.json` must not already be in use by something else.

and to sync:

```sh
hesync sync
```

## Automatic sync

[`contrib/hesync-helium`](contrib/hesync-helium) starts Helium, syncing before it opens and after it closes.
To launch Helium through it from the app menu:

```sh
curl -fsSLo ~/.local/bin/hesync-helium https://raw.githubusercontent.com/etams20/go-helium-sync/main/contrib/hesync-helium
chmod +x ~/.local/bin/hesync-helium
sed "s|^Exec=helium %U|Exec=$HOME/.local/bin/hesync-helium %U|" \
  /usr/share/applications/helium.desktop > ~/.local/share/applications/helium.desktop
```

This overrides the system entry. Typing `helium` in a terminal is unaffected.

To undo: `rm ~/.local/share/applications/helium.desktop ~/.local/bin/hesync-helium`.

## Releasing

```sh
git tag v0.1.0 && git push origin v0.1.0
```

## Tests

```sh
go test ./...
```
