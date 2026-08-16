# Installing StateRelay

StateRelay release archives include the `relay` binary, the license, the README, a version file, and a small install script.

## macOS and Linux

Download the archive for your platform, extract it, and run:

```bash
./install.sh
```

By default, the script installs to `/usr/local/bin/relay`. To install somewhere else:

```bash
PREFIX="$HOME/.local" ./install.sh
```

Make sure `$HOME/.local/bin` is on your `PATH` if you use a custom prefix.

## Windows

Download the Windows archive, extract it, and run PowerShell from the extracted folder:

```powershell
.\install.ps1
```

By default, the script installs to:

```text
%LOCALAPPDATA%\StateRelay\bin
```

To also add that directory to your user `PATH`:

```powershell
.\install.ps1 -AddToPath
```

Open a new terminal after changing `PATH`.

## Verify

After installation:

```bash
relay version
relay doctor --path .
```
