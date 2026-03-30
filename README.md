work in progress, still no room key rotation, and user invites also need work to be more secure

## Running locally

```
docker compose up -d   # start Postgres
go run ./broker/       # start the broker (config.yml is pre-configured)
go run ./shellapp/     # start the shell client
```

## Building

### macOS
```
brew install opus
go build ./...
```

### Linux
```
apt install libopus-dev   # Debian/Ubuntu
# or: dnf install opus-devel
go build ./...
```

### Windows
Requires MSYS2 for the C toolchain and libopus (needed by the voice package).

1. Install MSYS2 from https://www.msys2.org
2. Open the MSYS2 MinGW terminal and run:
   ```
   pacman -S mingw-w64-x86_64-gcc mingw-w64-x86_64-opus mingw-w64-x86_64-pkg-config
   ```
3. Add `C:\msys64\mingw64\bin` to your PATH
4. `go build ./...`
