# OpenVPN 3 Go Control (Ubuntu)

Local web UI to start, disconnect, and inspect **your** OpenVPN 3 Linux session. Implemented using the Go standard library, without Python, Node.js, or external Go dependencies.

## Requirements

- Ubuntu with the `openvpn3` Linux CLI installed and working for your user.
- Go **1.22 or newer** installed to compile the project (Go is not required to run the resulting executable).
- A valid `.ovpn` file at `/home/armine/Downloads/armine.stoykova.ovpn`.

## Build a standalone executable

From the project directory:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o openvpn3-control .
```

For ARM64 Ubuntu, replace `GOARCH=amd64` with `GOARCH=arm64`.

This produces a Go executable without dynamically linked C libraries. The executable **still requires** the separately installed `openvpn3` command, the OpenVPN 3 service, and the `.ovpn` configuration: it does not bundle a VPN engine.

Optional verification:

```bash
file openvpn3-control
ldd openvpn3-control
```

`ldd` should report that it is not a dynamic executable.

## Run

```bash
./openvpn3-control
```

Open **http://127.0.0.1:8765** in a browser on the same Ubuntu computer.

To change the VPN config path:

```bash
OPENVPN_CONFIG=/path/to/your-file.ovpn ./openvpn3-control
```

To stop the web server, press `Ctrl+C` in its terminal. This does not disconnect an existing VPN session.

## Buttons

- **VPN-on**: checks for an existing matching session; if none exists, runs `openvpn3 session-start --config <file>`.
- **VPN-off**: runs `openvpn3 session-manage --session-path <matching-session> --disconnect` only for sessions whose OpenVPN 3 session-list output contains the full configured path.
- **VPN-status**: queries `openvpn3 sessions-list` on each request and displays the reported output. Auto-refreshes every 10 seconds.

OpenVPN 3 may require external browser authentication. Complete the login as directed by OpenVPN 3, then refresh the status. A created session is not necessarily a connected tunnel.

## Limitations and safety

- The server listens only on `127.0.0.1:8765`, not on the LAN or internet. It checks request Host and Origin for API requests.
- Run it as the **same Linux user** who owns the VPN sessions. Do not run it as root or expose it via a reverse proxy.
- Session ownership is identified from the configuration path printed by `openvpn3 sessions-list`. If your OpenVPN 3 version shows only a profile name or hides the full path, the tool deliberately will not disconnect that session. Inspect `openvpn3 sessions-list` and adapt the matching logic for your version before use.
- Connection state parsing is based on the human-readable CLI output, which varies by OpenVPN 3 version. A session without a clear `Connected` status is shown as `PENDING`, not assumed connected. Inspect the raw output shown in the UI.
- Errors include real CLI output (stdout/stderr) plus process errors.
- If the `openvpn3` command hangs, the process is terminated at the command timeout; a VPN session may still need manual inspection.
- If your `.ovpn` file includes secrets, keep it private and restrict file permissions.
- This application controls the **system OpenVPN tunnel**, not a browser-only proxy. The browser uses the OS routing established by OpenVPN, subject to the VPN configuration and routing policy.

## Troubleshooting

```bash
openvpn3 sessions-list
openvpn3 session-start --config /home/armine/Downloads/armine.stoykova.ovpn
```

If a login browser does not open automatically, follow your OpenVPN 3 CLI's `session-auth` instructions. Do not create a second VPN session while the first is awaiting authentication.
