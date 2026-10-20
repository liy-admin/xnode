# Machine Auto Node (Xboard plugin)

Lets an xnode machine that has no nodes create its first node automatically, so a
fresh server only needs the one-line install command.

## What it does

Adds `POST /api/v2/server/machine/auto-node`, authenticated with the machine's
`machine_id` + `token` (the same credentials as `/api/v2/server/machine/nodes`).

- If the machine already has **any** node (enabled or disabled), nothing is created
  and the existing node IDs are returned.
- Otherwise, when **Auto create** is on, it creates one **VLESS + Reality** node bound
  to the machine, with a freshly generated X25519 key pair and short ID.
- Protocol, groups, port, SNI, rate and name all come from the plugin settings. The
  node end can only suggest its public `host`; if it does not, the request's source
  IP is used.

| Response | Meaning |
|---|---|
| `201 {"created":true,"node_ids":[id]}` | Node created |
| `200 {"created":false,"node_ids":[...]}` | Machine already has nodes, or auto create is off |
| `403` | Wrong machine id / token, or machine disabled |
| `404` | Plugin not installed or not enabled |
| `409` | Another request for the same machine is in progress |
| `422` | `group_ids` not set, bad port, or invalid `host` |

## Install

1. Zip the `MachineAutoNode` directory (the zip must contain `MachineAutoNode/config.json`).
2. Admin panel → Plugins → Upload, then install and enable **Machine Auto Node**.
3. Open its settings: turn on **允许自动创建节点**, fill **权限组 ID** (e.g. `1,2`),
   and adjust port / SNI / name if needed.

Then install xnode on a server in machine mode. On first start with no nodes it
calls this endpoint, and the new node starts within seconds.

## Security notes

- The machine token can create at most one node, only on its own machine, and only
  while that machine has no nodes. Disabling the auto-created node does not cause a
  new one to be created.
- Settings are read straight from the database on every call, so changes apply
  immediately (Xboard's plugin config cache is bypassed).
- The created node is a normal Xboard node; edit or delete it in the admin panel as usual.
