#!/usr/bin/env python3
"""Exercise the installer without network access or changes to the user's home."""

import io
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import unittest


INSTALLER = Path(__file__).resolve().parents[1] / "install.sh"
REPO = "https://github.com/dhakad22klx/justsay"
BINARY = b'#!/bin/sh\n[ "$#" = 0 ] || exit 1\nprintf "justsay test agent\\n"\n'


class InstallerTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="justsay-test-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.home = self.root / "home with spaces"
        self.home.mkdir()
        self.bin = self.root / "bin"
        self.bin.mkdir()
        self.fixture = self.root / "release-binary"
        self.fixture.write_bytes(BINARY)
        self.archive = self.root / "release.tar.gz"
        with tarfile.open(self.archive, "w:gz") as archive:
            for name, data in (("go.mod", b"module justsay-harness\n\ngo 1.26.5\n"),
                               ("go.sum", b""), ("main.go", b"package main\n")):
                entry = tarfile.TarInfo("justsay-release/" + name)
                entry.size = len(data)
                archive.addfile(entry, io.BytesIO(data))
        # Go and HTTP are mocked; no network or real module cache is used.
        for command in ("awk", "cp", "chmod", "mv", "mkdir", "mktemp", "rm", "tar", "gzip",
                        "grep", "dirname", "sha256sum", "sh"):
            self.bin.joinpath(command).symlink_to(shutil.which(command))
        self.executable("uname", '#!/bin/sh\ncase "$1" in\n'
                        ' -s) echo "$TEST_OS";;\n -m) echo "$TEST_ARCH";;\nesac\n')
        self.executable("go", '''#!/bin/sh
case "$*" in
    version) printf 'go version go%s linux/amd64\\n' "${TEST_GO_VERSION:-1.26.5}" ;;
    'mod download')
        [ -f go.mod ] && [ -f go.sum ] || exit 1
        [ "$GOWORK" = off ] && [ "$GOTOOLCHAIN" = local ] || exit 1
        [ "${TEST_GO_FAIL:-}" != 1 ] || exit 1
        printf '%s\\n' "$PWD" >> "$TEST_GO_DOWNLOADS"
        ;;
    build*)
        [ "${TEST_BUILD_FAIL:-}" != 1 ] || exit 1
        [ -f go.mod ] && [ "$CGO_ENABLED" = 0 ] || exit 1
        printf '%s/%s\\n' "$GOOS" "$GOARCH" >> "$TEST_GO_BUILDS"
        while [ "$#" -gt 0 ]; do
            if [ "$1" = -o ]; then cp "$TEST_BINARY" "$2"; exit; fi
            shift
        done
        exit 1
        ;;
    *) exit 1 ;;
esac
''')
        self.executable("curl", """#!/usr/bin/python3
import hashlib, os, pathlib, sys
args = sys.argv[1:]
url = args[-1]
with open(os.environ['TEST_REQUESTS'], 'a') as log:
    log.write(url + '\\n')
if os.environ.get('TEST_FAIL') and url.endswith(os.environ['TEST_FAIL']):
    if '-w' in args:
        print('500', end='')
    sys.exit(22)
repo = 'https://github.com/dhakad22klx/justsay'
tag = os.environ.get('TEST_TAG', 'main-test')
if url == repo + '/releases/latest':
    print(repo + '/releases/tag/' + tag, end='')
    sys.exit(0)
output = pathlib.Path(args[args.index('-o') + 1])
if url == repo + '/archive/refs/tags/' + tag + '.tar.gz':
    output.write_bytes(pathlib.Path(os.environ['TEST_ARCHIVE']).read_bytes())
    sys.exit(0)
assert url.startswith(repo + '/releases/download/' + tag + '/'), url
if (os.environ.get('TEST_NO_BINARY') and '/justsay_' in url) or (
        os.environ.get('TEST_NO_CHECKSUM') and url.endswith('/checksums.txt')):
    print('404', end='')
    sys.exit(22)
binary = pathlib.Path(os.environ['TEST_BINARY']).read_bytes()
files = {'go.mod': b'module justsay-harness\\n\\ngo 1.26.5\\n', 'go.sum': b''}
assets = ('justsay_linux_amd64', 'justsay_linux_arm64',
          'justsay_darwin_amd64', 'justsay_darwin_arm64')
files.update({asset: binary for asset in assets})
if url.endswith('/checksums.txt'):
    output.write_text(''.join(
        ('0' * 64 if os.environ.get('TEST_CORRUPT') else hashlib.sha256(data).hexdigest())
        + '  ' + name + '\\n' for name, data in files.items()))
else:
    output.write_bytes(files[url.rsplit('/', 1)[-1]])
if '-w' in args:
    print('200', end='')
""")
        self.env = {"HOME": str(self.home), "PATH": str(self.bin),
                    "SHELL": "/bin/bash", "TMPDIR": str(self.root),
                    "TEST_OS": "Linux", "TEST_ARCH": "x86_64",
                    "TEST_BINARY": str(self.fixture),
                    "TEST_ARCHIVE": str(self.archive),
                    "TEST_GO_DOWNLOADS": str(self.root / "go-downloads"),
                    "TEST_GO_BUILDS": str(self.root / "go-builds"),
                    "TEST_REQUESTS": str(self.root / "requests")}

    def executable(self, name, text):
        path = self.bin / name
        path.write_text(text)
        path.chmod(0o755)

    def install(self, success=True, shell="/bin/sh", piped=False):
        if piped:
            command = ["/bin/sh", "-c", 'cat "$1" | "$2"', "--", str(INSTALLER), shell]
            # cat is needed only to reproduce the documented pipe invocation.
            if not (self.bin / "cat").exists():
                self.bin.joinpath("cat").symlink_to(shutil.which("cat"))
        else:
            command = [shell, str(INSTALLER)]
        result = subprocess.run(command, env=self.env, text=True, capture_output=True, timeout=15)
        self.assertEqual(result.returncode == 0, success, result.stdout + result.stderr)
        self.assertFalse(list(self.root.glob("justsay.*")), "temporary downloads leaked")
        self.assertFalse(list((self.home / ".local/bin").glob(".justsay.*")), "staged binaries leaked")
        return result

    def test_platforms_and_pipe_invocations(self):
        for os_name, arch, asset in (
            ("Linux", "x86_64", "linux_amd64"),
            ("Linux", "aarch64", "linux_arm64"),
            ("Darwin", "x86_64", "darwin_amd64"),
            ("Darwin", "arm64", "darwin_arm64"),
        ):
            for shell in ("/bin/sh", "/bin/bash"):
                with self.subTest(os=os_name, arch=arch, shell=shell):
                    self.env.update(TEST_OS=os_name, TEST_ARCH=arch)
                    result = self.install(shell=shell, piped=True)
                    self.assertIn("Verified installed executable", result.stdout)
                    self.assertIn("export PATH=", result.stdout)
                    requests = Path(self.env["TEST_REQUESTS"]).read_text().splitlines()
                    self.assertIn(f"{REPO}/releases/download/main-test/justsay_{asset}", requests[-4:])
                    self.assertTrue(Path(self.env["TEST_GO_DOWNLOADS"]).is_file())
                    self.assertEqual((self.home / ".local/bin/justsay").read_bytes(), BINARY)

    def test_path_persists_without_duplicate_blocks(self):
        profile = self.home / ".bash_profile"
        profile.write_text("# user's existing setup\n")
        self.install()
        before = profile.read_text()
        self.install()
        self.assertEqual(profile.read_text(), before)
        self.assertTrue(before.startswith("# user's existing setup\n"))
        for startup in (".bashrc", ".bash_profile"):
            result = subprocess.run(
                ["/bin/sh", "-c", '. "$1"; command -v justsay; justsay', "--", str(self.home / startup)],
                env=self.env, text=True, capture_output=True, timeout=15)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn(str(self.home / ".local/bin/justsay"), result.stdout)

    def test_shell_startup_locations(self):
        for shell, filenames in (
            ("/bin/zsh", (".zshrc", ".zprofile")),
            ("/bin/sh", (".profile",)),
            ("/bin/fish", (".config/fish/conf.d/justsay.fish",)),
        ):
            with self.subTest(shell=shell):
                self.env["SHELL"] = shell
                self.install()
                for filename in filenames:
                    self.assertTrue((self.home / filename).is_file())
        self.env.update(SHELL="/bin/zsh", ZDOTDIR=str(self.home / "custom zsh"))
        self.install()
        self.assertTrue((self.home / "custom zsh/.zshrc").is_file())
        self.env.update(SHELL="/bin/fish", XDG_CONFIG_HOME=str(self.home / "custom config"))
        self.install()
        self.assertTrue((self.home / "custom config/fish/conf.d/justsay.fish").is_file())

    def test_unset_shell_and_path_already_present(self):
        del self.env["SHELL"]
        self.env["PATH"] = str(self.home / ".local/bin") + ":" + str(self.bin)
        result = self.install()
        self.assertTrue((self.home / ".profile").exists())
        self.assertNotIn("Open a new terminal", result.stdout)

    def test_shasum_fallback(self):
        shasum = shutil.which("shasum")
        if not shasum:
            self.skipTest("shasum is not installed")
        (self.bin / "sha256sum").unlink()
        self.bin.joinpath("shasum").symlink_to(shasum)
        self.install()

    def test_failures_preserve_existing_install(self):
        installed = self.home / ".local/bin/justsay"
        installed.parent.mkdir(parents=True)
        installed.write_text("old installation")
        for settings, message in (
            ({"TEST_FAIL": "/releases/latest"}, "Cannot find the latest release"),
            ({"TEST_FAIL": "/justsay_linux_amd64"}, "Cannot download justsay_linux_amd64"),
            ({"TEST_FAIL": "/checksums.txt"}, "Cannot download checksums.txt"),
            ({"TEST_FAIL": ".tar.gz"}, "Cannot download the source"),
            ({"TEST_CORRUPT": "1"}, "Checksum mismatch"),
            ({"TEST_GO_FAIL": "1"}, "Cannot install Go dependencies"),
            ({"TEST_NO_BINARY": "1", "TEST_BUILD_FAIL": "1"}, "Cannot build release"),
        ):
            with self.subTest(settings=settings):
                self.env.update(settings)
                result = self.install(success=False)
                self.assertIn(message, result.stderr)
                self.assertEqual(installed.read_text(), "old installation")
                for key in settings:
                    del self.env[key]
    def test_unsupported_platforms(self):
        for settings, message in (({"TEST_OS": "MINGW64_NT"}, "Unsupported OS"),
                                  ({"TEST_ARCH": "armv7l"}, "Unsupported architecture")):
            with self.subTest(settings=settings):
                saved = self.env.copy()
                self.env.update(settings)
                self.assertIn(message, self.install(success=False).stderr)
                self.assertFalse(Path(self.env["TEST_REQUESTS"]).exists())
                self.env = saved

    def test_missing_prerequisites(self):
        del self.env["HOME"]
        self.assertIn("HOME is not set", self.install(success=False).stderr)
        self.env["HOME"] = str(self.home)
        (self.bin / "go").unlink()
        self.assertIn("Go is required", self.install(success=False).stderr)
        (self.bin / "curl").unlink()
        self.assertIn("curl is required", self.install(success=False).stderr)

    def test_go_version_requirement(self):
        for version, success in (("1.26.4", False), ("1.25.9", False),
                                 ("1.26.5", True), ("1.26.10", True), ("1.27.0", True)):
            with self.subTest(version=version):
                self.env["TEST_GO_VERSION"] = version
                result = self.install(success=success)
                self.assertIn("Go 1.26.5 or later is required", result.stdout + result.stderr)

    def test_source_only_release_builds_for_detected_platform(self):
        self.env.update(TEST_NO_BINARY="1", TEST_OS="Darwin", TEST_ARCH="arm64")
        result = self.install()
        self.assertIn("building release main-test with Go", result.stdout)
        self.assertEqual(Path(self.env["TEST_GO_BUILDS"]).read_text(), "darwin/arm64\n")
        self.assertEqual((self.home / ".local/bin/justsay").read_bytes(), BINARY)

    def test_binary_without_published_checksums(self):
        self.env["TEST_NO_CHECKSUM"] = "1"
        self.install()
        self.assertFalse(Path(self.env["TEST_GO_BUILDS"]).exists())

    def test_new_latest_release_is_resolved_each_time(self):
        self.install()
        self.env["TEST_TAG"] = "v2.0"
        self.install()
        requests = Path(self.env["TEST_REQUESTS"]).read_text().splitlines()
        self.assertEqual(requests.count(REPO + "/releases/latest"), 2)
        self.assertIn(REPO + "/archive/refs/tags/v2.0.tar.gz", requests)
        self.assertIn(REPO + "/releases/download/v2.0/justsay_linux_amd64", requests)
        self.assertFalse(any("/main/" in url for url in requests))


if __name__ == "__main__":
    unittest.main(verbosity=2)
