# Third-Party Notices

This distribution includes third-party software in addition to go-music-dl.
Each component remains subject to its own license; the AGPL-3.0 license for
go-music-dl does not replace those licenses.

The final container is based on Alpine Linux 3.22 and installs `ffmpeg`,
`ca-certificates`, and `tzdata` with Alpine's package manager. Exact installed
package versions and declared licenses are recorded in `/lib/apk/db/installed`
inside the image. Alpine package metadata and corresponding source packages are
available from https://pkgs.alpinelinux.org/ and https://gitlab.alpinelinux.org/alpine/aports.

FFmpeg licensing and source information are available from
https://ffmpeg.org/legal.html and https://ffmpeg.org/download.html.

The Go and Rust dependency manifests are `go.mod`, `go.sum`,
`desktop/Cargo.toml`, and `desktop/Cargo.lock`. Their corresponding source code
and license notices are available from the module and crate sources identified
by those manifests. The published container SBOM records the components found
during each image build.
