# hack/

Make fragments included by the root `Makefile` (`include hack/*.mk`).
No shell entrypoints here — recipes are written in Make.

| File | Role |
|------|------|
| `check.mk` | `test` / `vet` / `fmt` / `check` / `lint` |
| `build.mk` | `build-backend` / `build-one` |
| `bundle.mk` | `bundle` / `build` / `linux-amd64` / `linux-arm64` |
| `integration.mk` | `integration` → `dist/app` |
| `tar.mk` | `tar` → `dist/plugin.tar.gz` |
| `detect-os.mk` | `detect-os` |
| `clean.mk` | `clean` |
| `help.mk` | `help` |

Prefer `make <target>` from the repo root. Override vars as needed:

```bash
make build-backend VERSION=1.2.3
make bundle DEST=dist/linux-amd64 GOOS=linux GOARCH=amd64 FORCE_BUILD=1
```
