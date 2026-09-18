#!/usr/bin/env sh
set -eu

usage() {
  cat <<'EOF'
Usage: install.sh --version VERSION [--install-dir DIRECTORY]

Downloads the matching HarnessForge GitHub Release, verifies its SHA-256
checksum, and installs the harnessforge executable.
EOF
}

version="${HARNESSFORGE_VERSION:-}"
install_dir="${HARNESSFORGE_INSTALL_DIR:-${HOME}/.local/bin}"

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      [ "$#" -ge 2 ] || { echo "--version requires a value" >&2; exit 2; }
      version="$2"
      shift 2
      ;;
    --install-dir)
      [ "$#" -ge 2 ] || { echo "--install-dir requires a value" >&2; exit 2; }
      install_dir="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if ! printf '%s\n' "$version" | awk 'match($0, /^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$/) && substr($0, RSTART, RLENGTH) == $0 { found=1 } END { exit(found ? 0 : 1) }'; then
  echo "Version must be a semantic version such as 1.2.3." >&2
  exit 2
fi

os="${HARNESSFORGE_OS_OVERRIDE:-$(uname -s)}"
arch="${HARNESSFORGE_ARCH_OVERRIDE:-$(uname -m)}"
case "$os" in
  Darwin) goos=darwin ;;
  Linux) goos=linux ;;
  *) echo "Unsupported operating system: $os (supported: Darwin, Linux)." >&2; exit 1 ;;
esac
case "$arch" in
  x86_64|amd64) goarch=amd64 ;;
  arm64|aarch64) goarch=arm64 ;;
  *) echo "Unsupported architecture: $arch (supported: amd64, arm64)." >&2; exit 1 ;;
esac

base_url="${HARNESSFORGE_RELEASE_BASE_URL:-https://github.com/joicepassos/harness-forge/releases/download}"
case "$base_url" in
  https://*|http://127.0.0.1:*|http://localhost:*) ;;
  *) echo "Release base URL must use HTTPS." >&2; exit 1 ;;
esac
case "$base_url" in
  http://*)
    if command -v curl >/dev/null 2>&1; then
      download() { curl --fail --silent --show-error --location "$1" --output "$2"; }
    elif command -v wget >/dev/null 2>&1; then
      download() { wget --output-document="$2" "$1"; }
    else
      echo "Install curl or wget and run this installer again." >&2
      exit 1
    fi
    ;;
  *)
    if command -v curl >/dev/null 2>&1; then
      download() { curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 "$1" --output "$2"; }
    elif command -v wget >/dev/null 2>&1; then
      download() { wget --https-only --secure-protocol=TLSv1_2 --output-document="$2" "$1"; }
    else
      echo "Install curl or wget and run this installer again." >&2
      exit 1
    fi
    ;;
esac
release_url="${base_url%/}/v${version}"
archive="harnessforge_${version}_${goos}_${goarch}.tar.gz"
checksums="harnessforge_${version}_checksums.txt"

tmp_dir="$(mktemp -d 2>/dev/null || mktemp -d -t harnessforge)"
cleanup() { rm -rf "$tmp_dir"; }
trap cleanup EXIT HUP INT TERM
download "$release_url/$checksums" "$tmp_dir/$checksums"
download "$release_url/$archive" "$tmp_dir/$archive"
[ "$(wc -c < "$tmp_dir/$checksums")" -le 1048576 ] || { echo "Checksum manifest exceeds the supported size limit." >&2; exit 1; }
[ "$(wc -c < "$tmp_dir/$archive")" -le 67108864 ] || { echo "Release archive exceeds the supported download size limit." >&2; exit 1; }

expected="$(awk -v file="$archive" '$2 == file || $2 == "*" file { print tolower($1); exit }' "$tmp_dir/$checksums")"
[ "${#expected}" -eq 64 ] || { echo "Checksum manifest does not contain $archive." >&2; exit 1; }
if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$tmp_dir/$archive" | awk '{print tolower($1)}')"
else
  actual="$(shasum -a 256 "$tmp_dir/$archive" | awk '{print tolower($1)}')"
fi
[ "$actual" = "$expected" ] || { echo "Checksum mismatch for $archive; refusing to install." >&2; exit 1; }

extract_dir="$tmp_dir/extracted"
mkdir -p "$extract_dir"
if ! tar -tzf "$tmp_dir/$archive" > "$tmp_dir/entries"; then
  echo "Could not inspect the verified release archive." >&2
  exit 1
fi
if ! awk 'NF { count++; if ($0 != "harnessforge") { bad=1 } } END { exit (bad || count != 1) }' "$tmp_dir/entries"; then
  echo "Release archive contains unexpected paths." >&2
  exit 1
fi
tar -xzf "$tmp_dir/$archive" -C "$extract_dir" harnessforge
[ -f "$extract_dir/harnessforge" ] && [ ! -L "$extract_dir/harnessforge" ] || { echo "Release archive has no regular harnessforge executable." >&2; exit 1; }

mkdir -p "$install_dir"
destination="$install_dir/harnessforge"
temporary="$(mktemp "$install_dir/.harnessforge.XXXXXX")" || { echo "Could not create a safe temporary installer file." >&2; exit 1; }
trap 'rm -f "$temporary"; cleanup' EXIT HUP INT TERM
cp "$extract_dir/harnessforge" "$temporary"
chmod 0755 "$temporary"
mv -f "$temporary" "$destination"
echo "Installed HarnessForge $version to $destination"
