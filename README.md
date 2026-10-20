# xnode

Node backend for [Xboard](https://github.com/cedar2025/Xboard). Supports `sing-box` / `xray-core` dual kernels.

Based on [cedar2025/xboard-node](https://github.com/cedar2025/xboard-node). Kernels are built from self-maintained forks, each an official upstream tag plus reviewed patches: [liy-admin/sing-box](https://github.com/liy-admin/sing-box) and [liy-admin/Xray-core](https://github.com/liy-admin/Xray-core). Release binaries are published with a `SHA256SUMS` file, which the installer and `xbctl upgrade` verify before installing.

> **Disclaimer**: This project is for educational and learning purposes only.

## Features

- Protocols: V2Ray family, Trojan, Shadowsocks, Hysteria2, TUIC, AnyTLS
- Sync: WebSocket push + REST polling dual channel
- User controls: speed limit, device limit, alive-IP tracking, hot update
- Deploy modes: node mode, machine mode, standalone mode
- Multi-instance: single process binding multiple panels / nodes

## Install

### Docker

```bash
docker run -d --restart=always --network=host \
  -e apiHost=https://panel.com -e apiKey=TOKEN -e nodeID=1 \
  ghcr.io/liy-admin/xnode:latest
```

### Installer (Linux systemd)

```bash
# Node mode
curl -fsSL https://raw.githubusercontent.com/liy-admin/xnode/dev/install.sh | \
  sudo bash -s -- --mode node --panel https://panel.example.com --token TOKEN --node-id 1

# Machine mode
curl -fsSL https://raw.githubusercontent.com/liy-admin/xnode/dev/install.sh | \
  sudo bash -s -- --mode machine --panel https://panel.example.com --token TOKEN --machine-id 1
```

### Auto-create the first node (machine mode)

With the [Machine Auto Node](panel-plugin/MachineAutoNode/README.md) panel plugin, a new
server only needs the machine-mode command above: if the machine has no nodes yet, the
panel creates a VLESS + Reality node for it and the server starts serving right away.

1. Download `MachineAutoNode.zip` from the [latest release](https://github.com/liy-admin/xnode/releases/latest)
   and upload it in the admin panel → Plugins, then install and enable it.
2. In the plugin settings, turn on auto create and fill in the group IDs (port, SNI and
   name template are optional).
3. Run the machine-mode install command. Behind NAT, add `--node-host <public IP or domain>`;
   to opt out, add `--no-auto-node`.

The panel only creates a node while the machine has none (disabled nodes count), and the
plugin settings, not the server, decide protocol, groups and port. Panels without the
plugin are unaffected: the node checks once, logs a hint, and stops asking.

## xbctl

Run `xbctl` after installation for help. Common commands:

```bash
xbctl list                          # list all instances
xbctl status                        # running status
xbctl bind add-node --panel URL --token TOKEN --node-id 1
xbctl bind add-machine --panel URL --token TOKEN --machine-id 1
xbctl bind remove-node --panel URL --node-id 1
xbctl service restart
```

## Configuration

Legacy single-panel config is fully compatible. Appending bindings auto-migrates to `instances` format. See `config.yml.example`.

## Extensions

- Custom routes: [docs-custom-routes.md](docs-custom-routes.md)
- Custom outbounds: [docs-custom-outbounds.md](docs-custom-outbounds.md)
- DNS providers (ACME DNS-01): [docs-dns-providers.md](docs-dns-providers.md)

## License

MPL-2.0.
