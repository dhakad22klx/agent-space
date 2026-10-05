#!/bin/sh
# Install the latest published JustSay release. Compatible with sh and bash.
set -eu

log() { printf 'justsay: %s\n' "$*"; }
die() { printf 'justsay: error: %s\n' "$*" >&2; exit 1; }

[ -n "${HOME:-}" ] || die 'HOME is not set.'
command -v curl >/dev/null 2>&1 || die 'curl is required. Install curl and try again.'
command -v go >/dev/null 2>&1 || die 'Go is required. Install Go from https://go.dev/dl/ and try again.'
command -v tar >/dev/null 2>&1 || die 'tar is required to extract the release source.'
go_version=$(go version 2>&1) || die "Cannot run 'go version': $go_version"
log "$go_version"

case "$(uname -s)" in
    Linux) os=linux ;;
    Darwin) os=darwin ;;
    *) die 'Unsupported OS. JustSay supports Linux and macOS (including WSL Linux).' ;;
esac
case "$(uname -m)" in
    x86_64|amd64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) die 'Unsupported architecture. JustSay supports x86_64 and ARM64.' ;;
esac

if command -v sha256sum >/dev/null 2>&1; then
    checksum_tool=sha256sum
elif command -v shasum >/dev/null 2>&1; then
    checksum_tool=shasum
else
    die 'sha256sum or shasum is required to verify the download.'
fi

repo_url=https://github.com/dhakad22klx/justsay
asset="justsay_${os}_${arch}"
install_dir="$HOME/.local/bin"
tmp_dir=$(mktemp -d "${TMPDIR:-/tmp}/justsay.XXXXXX") || die 'Cannot create a temporary directory.'
staged_binary=
cleanup() {
    rm -rf "$tmp_dir"
    if [ -n "$staged_binary" ]; then rm -f "$staged_binary"; fi
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

# Resolve once, so source, dependencies, and any binary use the same release.
log "Finding the latest release for $os/$arch..."
release_url=$(curl -fsSLI --connect-timeout 15 --max-time 60 \
    -o /dev/null -w '%{url_effective}' "$repo_url/releases/latest") \
    || die "Cannot find the latest release. Check your connection and $repo_url/releases."
case "$release_url" in
    "$repo_url/releases/tag/"*) tag=${release_url#"$repo_url/releases/tag/"} ;;
    *) die 'GitHub did not return a release. No published release may be available yet.' ;;
esac
[ -n "$tag" ] || die 'GitHub returned an empty release tag.'
download_url="$repo_url/releases/download/$tag"

log "Downloading release $tag..."
curl -fsSL --connect-timeout 15 --max-time 300 \
    -o "$tmp_dir/source.tar.gz" "$repo_url/archive/refs/tags/$tag.tar.gz" \
    || die "Cannot download the source for release $tag. Check your connection to GitHub."
mkdir "$tmp_dir/source" || die 'Cannot create the source directory.'
tar -xzf "$tmp_dir/source.tar.gz" -C "$tmp_dir/source" --strip-components=1 \
    || die 'Cannot extract the release source.'
[ -f "$tmp_dir/source/go.mod" ] || die "Release $tag does not contain go.mod."

required_go=$(awk '$1 == "go" { print $2 }' "$tmp_dir/source/go.mod")
installed_go=$(printf '%s\n' "$go_version" | awk '{ sub(/^go/, "", $3); print $3 }')
case "$required_go" in ''|*[!0-9.]*) die 'Release go.mod has an invalid Go version.' ;; esac
log "Go $required_go or later is required."
case "$installed_go" in
    ''|*[!0-9.]*) die "Cannot determine a stable Go version from: $go_version" ;;
esac
if ! awk -v installed="$installed_go" -v required="$required_go" 'BEGIN {
    split(installed, current, "."); split(required, minimum, ".")
    for (i = 1; i <= 3; i++) {
        if (current[i] + 0 > minimum[i] + 0) exit 0
        if (current[i] + 0 < minimum[i] + 0) exit 1
    }
}' </dev/null; then
    die "Go $required_go or later is required; found Go $installed_go. Update Go from https://go.dev/dl/ and try again."
fi
log 'Installing Go dependencies (go mod download)...'
(cd "$tmp_dir/source" && GOWORK=off GOTOOLCHAIN=local go mod download) \
    || die 'Cannot install Go dependencies. Check your network and Go module cache permissions, then try again.'

# Prefer a matching binary. Source-only releases work without special assets.
if binary_status=$(curl -fsSL --connect-timeout 15 --max-time 300 \
    -o "$tmp_dir/$asset" -w '%{http_code}' "$download_url/$asset" 2>"$tmp_dir/curl-error"); then
    log "Downloaded $asset"
    if checksum_status=$(curl -fsSL --connect-timeout 15 --max-time 60 \
        -o "$tmp_dir/checksums.txt" -w '%{http_code}' "$download_url/checksums.txt" 2>"$tmp_dir/curl-error"); then
        expected=$(awk -v asset="$asset" '$2 == asset { print $1 }' "$tmp_dir/checksums.txt")
        [ "${#expected}" -eq 64 ] || die "Missing or invalid SHA-256 checksum for $asset."
        case "$expected" in *[!0-9a-fA-F]*) die "Invalid SHA-256 checksum for $asset." ;; esac
        if [ "$checksum_tool" = sha256sum ]; then
            actual=$(sha256sum "$tmp_dir/$asset") || die 'Cannot calculate the download checksum.'
        else
            actual=$(shasum -a 256 "$tmp_dir/$asset") || die 'Cannot calculate the download checksum.'
        fi
        actual=${actual%% *}
        [ "$actual" = "$expected" ] || die 'Checksum mismatch. Installation stopped; try again.'
        log 'Verified release checksum.'
    elif [ "$checksum_status" != 404 ]; then
        die "Cannot download checksums.txt from $tag. Check your connection to GitHub."
    fi
elif [ "$binary_status" = 404 ]; then
    log "No $asset asset; building release $tag with Go..."
    (cd "$tmp_dir/source" && GOWORK=off GOTOOLCHAIN=local CGO_ENABLED=0 \
        GOOS="$os" GOARCH="$arch" go build -buildvcs=false -trimpath -o "$tmp_dir/$asset" .) \
        || die "Cannot build release $tag. Check the Go build errors above."
else
    die "Cannot download $asset from $tag. Check your connection to GitHub."
fi

# Stage the completed binary before replacing an existing installation.
mkdir -p "$install_dir" || die "Cannot create $install_dir. Check directory permissions."
[ ! -d "$install_dir/justsay" ] || die "$install_dir/justsay is a directory. Move it before installing."
staged_binary=$(mktemp "$install_dir/.justsay.XXXXXX") || die "Cannot write to $install_dir."
cp "$tmp_dir/$asset" "$staged_binary" || die 'Cannot stage the binary.'
chmod 755 "$staged_binary" || die 'Cannot make the binary executable.'
mv -f "$staged_binary" "$install_dir/justsay" || die "Cannot install to $install_dir/justsay."
staged_binary=
log "Installed $install_dir/justsay"

path_block='# >>> JustSay PATH >>>
case ":$PATH:" in
    *":$HOME/.local/bin:"*) ;;
    *) export PATH="$HOME/.local/bin:$PATH" ;;
esac
# <<< JustSay PATH <<<'
add_path() {
    profile=$1
    if [ -f "$profile" ] && grep -Fq '# >>> JustSay PATH >>>' "$profile"; then
        return
    fi
    mkdir -p "$(dirname "$profile")" || die "Cannot create the directory for $profile."
    printf '\n%s\n' "$path_block" >> "$profile" || die "Cannot update $profile. Add ~/.local/bin to PATH manually."
    log "Added PATH setup to $profile"
}

shell_name=${SHELL:-sh}
shell_name=${shell_name##*/}
case "$shell_name" in
    bash)
        add_path "$HOME/.bashrc"
        if [ -f "$HOME/.bash_profile" ]; then
            add_path "$HOME/.bash_profile"
        elif [ -f "$HOME/.bash_login" ]; then
            add_path "$HOME/.bash_login"
        else
            add_path "$HOME/.profile"
        fi
        ;;
    zsh)
        add_path "${ZDOTDIR:-$HOME}/.zshrc"
        add_path "${ZDOTDIR:-$HOME}/.zprofile"
        ;;
    fish)
        path_block='# >>> JustSay PATH >>>
if not contains -- "$HOME/.local/bin" $PATH
    set -gx PATH "$HOME/.local/bin" $PATH
end
# <<< JustSay PATH <<<'
        add_path "${XDG_CONFIG_HOME:-$HOME/.config}/fish/conf.d/justsay.fish"
        ;;
    sh|dash|ksh|'') add_path "$HOME/.profile" ;;
    *) log "Shell $shell_name needs manual PATH setup; see INSTALL.md." ;;
esac

# A piped installer cannot modify the parent shell's environment.
case ":$PATH:" in
    *":$install_dir:"*) ;;
    *)
        log 'Open a new terminal, or run this in your current shell:'
        if [ "$shell_name" = fish ]; then
            printf '  set -gx PATH "$HOME/.local/bin" $PATH\n'
        else
            printf '  export PATH="$HOME/.local/bin:$PATH"\n'
        fi
        ;;
esac
[ -x "$install_dir/justsay" ] || die 'Installed binary is not executable.'
log 'Verified installed executable.'
log "Ready. Run 'justsay' to start the agent."
