# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **A block can be put next to a named block instead of at a number.** A config
  that writes into a group may say `after: <name>` or `before: <name>`, naming
  another block of the same sorted column; the block is placed there and the
  numbers of everything around it stop mattering.

  A priority is a poor contract between a topology and a module: it holds only
  while both agree what the numbers mean, and the module cannot move its own
  blocks afterwards without breaking the topology. A name is the module's to
  keep.

  Where it would mean something else, it is refused: on a config that writes
  into no group, together with `priority:` on the same config, and on a name no
  config template carries. An anchor that is not in this particular column
  places nothing rather than failing — a block cannot know which objects its
  anchor is generated for. Blocks placed in a circle are reported.

  Three things now order config, and they are not interchangeable: `depends:`
  says what has to be generated first, `blocks: before/after` merges other
  blocks into this template's output, and `after:`/`before:` place a block
  within a sorted column. The last only reads on a config with `group:`.

- **One sorter can gather several groups.** A sort-style config may say
  `sort_groups: [private, common]` in place of `sort_group:`. The blocks of
  every group named are put in one column and ordered together, so a group is
  where blocks are written from rather than a section of the result.

  Who may write a block and where the block ends up are different questions: a
  group only its own writer knows the name of and a group anything may write
  into can feed one file, which a single name cannot express.

### Fixed

- **Blocks of equal priority come out in the order their classes are declared.**
  Past ten config templates on one object they did not: the dependency graph
  sorts its node ids as strings, so `template_10` was placed before
  `template_2`. Ten is easily reached - the candidates are every class's
  templates and a module adds its own - so the order was scrambled in any real
  topology. None of the bundled topologies change, since none of them leans on
  it.

  Where the order between two blocks carries meaning, say it with `priority:`
  or `after:`/`before:` rather than by the order they are written: a module's
  classes are added after the topology's, so a tie puts the topology's block
  first, which is the opposite of what a hook wants.

- **A `group:` no sorter collects is now an error instead of a block that
  disappears.** The block was generated and then dropped, because the blocks are
  kept per sorter and a group with no sorter has nothing to hand them to. A
  typo in either name took a section out of a generated file with nothing said;
  the message now names the group written and the ones that are sorted.

### Removed

- **`topologies/kathara_basic`.** It was written when the Kathara module was, to
  have something that exercised it; every topology under `topologies/` generates
  for Kathara now, so nothing was left that only it showed. A shared medium and a
  point-to-point link are in `topologies/aggregate_crossing` and
  `topologies/ospf_multihost`, and what Kathara will not accept — interfaces it
  has not named, device names over thirty characters — is in `example/naming` and
  the wiki.

  It also read as though Kathara needed a topology of its own when containerlab
  and TiNET did not, which was never true.

## [0.8.0] - 2026-08-18

### Breaking changes

Read these before upgrading a topology from 0.7.x. Each is described in full
further down.

- **Generated files move.** A file declared with `path: /etc/frr/frr.conf` is
  written to `r1/etc/frr/frr.conf`, not `r1/frr.conf`. Anything reading
  generated files by path has to follow.
- **Interfaces are named `eth0`, `eth1`, ...** instead of `net0`, `net1`. A
  template naming an interface literally has to be updated.
- **A bridge is called something else on the machine.** What a topology names
  `sw1` reaches the machine as `br-a1b2c3`, and the veth reaching it as
  `eth0-a1b2c3`. Anything naming a bridge or its ports on the host — a script
  that cleans up after an interrupted run, a capture pinned to an interface —
  has to ask for the name rather than assume it: `{{ .clab_bridge }}` and
  `{{ .opp_clab_host_port }}` in a template, `dot2net data` from outside.

  The name is worked out from the lab's name and the node's, so the same inputs
  always give the same name and a tool that lost its files can rebuild it. **How
  it is worked out is therefore part of the interface**: changing the inputs, the
  length or the shape renames every bridge and orphans what an older version left
  behind, and will be released as a breaking change.
- **containerlab nodes get no management network by default.** A lab that relied on
  `clab exec`, the `clab-*` container names, or reachability through the
  management network has to turn it back on with
  `module_config.containerlab.management_network: true`.
- **A shared segment reaching across machines is replaced with one bridge per
  machine.** A multi-host topology that drew such a segment gets a different
  topology than before; `global.aggregate_crossing_links: false` restores it.
- **An unknown or duplicate key in the config file is an error.** A topology
  with a typo that used to run will now say so.
- **Two file definitions writing the same file is an error**, as is a config
  entry naming both `template` and `sourcefile`.
- **`virtual` no longer means "not deployed".** One flag used to answer two
  questions — is this object there, and does dot2net write its configuration —
  and a topology could not answer them separately. `deploy` now says what an
  object is materialised as, and `virtual: true` says only that its own
  configuration is withheld. **What to do:** write `deploy: none` wherever
  `virtual: true` meant "this is not deployed". You do not have to find them by
  reading: a class setting `virtual: true` without also naming a `deploy` is an
  error, so a 0.7.x topology stops on the first one. That error is a migration
  aid and is removed in 0.9.0.
- **A topology with its own `file:` entry for `<device>.startup` stops building
  once the Kathara module is loaded.** Two things share the word: the *file*
  `<device>.startup`, and the *config template* named `startup` that fills it.
  0.7.0 documented declaring the file yourself — a `file:` entry with
  `name_suffix: .startup` and `output: root` — and the Kathara module declares
  that file now, so both definitions write one file, which is an error.
  **What to do:** delete the `file:` entry. Keep the commands, moving them into a
  config template named `startup` if they are not there already; that template is
  what containerlab puts in `exec:` and TiNET in `cmds:`, so it is written once
  and runs on all three.
- **A template aggregating a segment's output has to name the layer.**
  `{{ .segments_<name> }}` becomes `{{ .segments_<layer>_<name> }}` — the layer
  was missing, so two layers using one config name were merged silently. **What
  to do:** add the layer. A template that still names the old parameter fails at
  build time with `map has no entry for key "segments_<name>"`, which is late
  enough in the run to look unrelated, so it is worth searching for before
  upgrading. No bundled topology referenced it; one out-of-tree topology did.
- **Most topologies move from `example/` to `topologies/`.** The directory held
  two different things: networks worth deploying, and small inputs written to
  demonstrate a piece of notation. Both are read by users and both are
  golden-tested, but only the first is worth running, and the shared name kept
  drawing new demonstrations into the same pile. The 13 deployable ones are now
  under `topologies/`; `example/` keeps the ones that demonstrate a notation.

  **Which one to open:** `topologies/` when you want a network to run, or one
  near yours to copy — the FRR topotests, the TiNET examples and dot2net's own,
  each generating for the platforms its readme names. `example/` when you want to
  see what one piece of the notation does; those are deliberately minimal and
  most configure no routing, so deploying one would show you little. A new
  topology belongs wherever it will be read from: something worth running goes in
  `topologies/`, something written to explain a notation goes in `example/`.

  Nothing inside a topology changed. **What to do:** update any path that names
  one: `cd example/ospf_simple` becomes `cd topologies/ospf_simple`.

### Removed

- **`example/vlan_multihost`.** It was rebuilt in this cycle to cover the
  multi-host output path, which then had no example at all. Two topologies now do
  that better — `topologies/ospf_multihost` is a network you can deploy, and
  `topologies/aggregate_crossing` shows what a segment reaching across machines
  costs — and the properties it was written to pin moved into unit tests.

  What it uniquely showed has a better home: a parameter assigned per segment is
  `example/param_share`, where the value is actually read back at both ends; the
  `assert` module is also loaded by `example/address_reservation`. Its own name
  had stopped being true, too — it configured no VLAN, only assigned a number and
  wrote it to a file.
- **`empty` on a config entry** and **`sort` on a `param_rule`**: both were
  declared and read by nothing, so writing either did exactly what leaving it out
  did. A file that has to exist whatever the conditions is written by a template
  that produces its empty content.
- **`--dir` on `build`**: declared, shown in `--help`, and read by nothing. Where
  a node's files go is decided by the file definitions and
  `global.output_group_class`.

### Changed (repository layout)

- **Topologies live under two roots.** `internal/test/example_test.go` walks
  both `topologies/` and `example/`, so every topology is golden-tested wherever
  it sits, and `tool/generate_expected.sh <name>` finds a topology under either.

### Added

- **Kathara module** (`module: [kathara]`): generates `lab.conf`, including each
  device's `image`, the directories to mount into it, and its
  `<device>.startup`. Kathara declares, per device, which collision domain each
  interface sits on; a collision domain is a shared medium, so the
  `deploy: platform` switch node that containerlab emits as a bridge and TiNET
  as a `switches:` entry becomes one here — named after the node, which itself
  gets no line since nothing is deployed for it. A link between two ordinary
  devices gets a domain of its own.

  A mounted file reaches a device through `volume`, which is Kathara's bind
  mount. Kathara mounts directories and refuses single files, so what is mounted
  is a directory the topology has named in `module_config.kathara.mount_dirs` —
  a statement that dot2net supplies everything the software needs in it, since
  mounting a directory hides what the image kept there and dot2net cannot see
  inside an image to know whether that is safe. A file to be mounted from a
  directory that was not named is reported, with both ways out: name the
  directory, or provide the file by copy.

  The lab lives in a directory of its own (`kathara/lab.conf`). Kathara reads a
  directory named after a device, next to `lab.conf`, as files to copy into that
  device once it has started — and the nodes' generated files sit at the output
  root under exactly such names. A `lab.conf` beside them would set that copying
  off: a second delivery nobody asked for, running after the device is up. One
  level down, the convention finds nothing.

  A node config template named `startup` becomes the file `<device>.startup`,
  from the same template containerlab puts in `exec:` and TiNET in `cmds:`. The
  startup commands are therefore written once and run on all three.

  Verified by deploying `topologies/ospf_simple` with the Kathara module added:
  the OSPF adjacency reaches Full and the routers two hops apart reach each
  other, from generated files alone and with the same image as containerlab.

  Two of Kathara's constraints are reported rather than worked around: an
  interface it lists in `lab.conf` under any name but `ethN` is rejected
  (Kathara names interfaces after their index there and offers no way to change
  it), as is a device name outside `[a-z0-9_]{1,30}`. The rule reaches only the
  interfaces Kathara names — a device the node's own configuration builds gets no
  line in `lab.conf`, so its name is the topology's to choose. A device whose
  interface indexes have a hole is also rejected, though automatic naming can no
  longer produce one.
- **A lab spanning machines gets one deployment file per machine — on all three
  platforms.** A lab is deployed to a single machine, so when a topology declares
  `worker` groups the deployment file becomes group-scoped: containerlab gets a
  `topo.yaml` per machine, TiNET a `spec.yaml`, Kathara a `lab.conf`. Each holds
  that machine's own nodes and the links with both ends inside it, and each can
  be deployed on its own. Paths inside it become relative to the machine's
  directory, which is why this requires `global.output_group_class: worker` so
  that a machine's files sit beside its deployment file; dot2net says so if they
  do not. Topologies that declare no placement unit keep the single
  network-scoped file.

  This is the platform-neutral half of multi-host support, and the point of
  building it for three platforms rather than one: the same DOT and YAML place a
  lab across machines whichever of them deploys it. What each platform writes
  differs and is described in its own entry below.

  **A link between two machines appears in no deployment file**, because nothing
  inside the platform can create it — the machines are wired outside the lab.
  Give each machine its own bridge and join them with a link; that link is what
  `boundary_crossing_connection_class` marks. Address assignment still sees one segment spanning
  both machines, because searching for a segment passes through bridges, which
  carry no addresses. `topologies/ospf_multihost` is built this way.
- **containerlab, per machine**: bind paths are written relative to the machine's
  directory, since containerlab resolves them against the directory holding the
  topology file.
- **TiNET, per machine**: mount paths are written relative to the machine's
  directory. Deployed on two VMs: the same DOT and YAML bring up an OSPF
  adjacency across the machine boundary on TiNET as well as on containerlab.
- **Kathara, per machine**: a lab is the directory holding `lab.conf` and the
  startup files, so each machine gets a directory of its own to run `kathara
  lstart` in. The entry script joins the same machine's group, so
  `./kathara.sh deploy` on each machine brings up its own half. Joining the
  machines needs one change to Kathara's own settings: its default collision
  domain driver puts no bridge on the host, so `network_plugin` has to be
  `kathara/katharanp` for a cross-machine link to have anything to attach to.
- **Every bundled topology under `topologies/` now generates for Kathara too.**
  `basic_clos` was the last that could not: it named its own interfaces (`up1`,
  `dn1`) on the ends of real links, and Kathara reads an interface's name from
  its index in `lab.conf`. The names carried nothing the topology depended on, so
  they were dropped; `example/address_reservation`, which had copied that graph,
  went with it.
- **`example/naming`**: what everything ends up called, and who decides it — a
  node renamed from its class prefix (`nodeautoname`), an interface named in the
  DOT file as `node:port`, another named from `interfaceclass.prefix`, and a
  connection from `connectionclass.prefix`. Naming was demonstrated only as a
  side effect of `basic_clos` before, and nowhere after it stopped.

  This is the one bundled topology that does not generate for Kathara, and it
  says why: **naming a link's interfaces and running on Kathara are mutually
  exclusive.**
- **`topologies/ospf_multihost`**: `topologies/ospf_simple` placed on two machines —
  the same OSPF configuration, split across a machine boundary. This is the
  one to copy when writing a multi-host topology. Deployed on two VMs: the OSPF adjacency between the
  border routers comes up across the boundary, each machine learns the other's
  subnets, and traffic is routed between them.
- **Entry point scripts** (`module_config.<module>.generate_scripts: true`): a
  `containerlab.sh`, `tinet.sh` or `kathara.sh` beside the lab, taking
  `deploy`, `destroy` and `exec <node> <command>...`. Each finds its own files,
  so it can be run from anywhere. `exec` gives back the command's own exit
  status, so a harness reading it sees what ran in the node rather than whether
  the script was happy.

  What it carries is the part that differs between platforms and is easy to get
  wrong: TiNET brings a lab up in two steps and its output is a shell script to
  be piped, with one line in it that is not a command; Kathara reads its files
  from the directory it runs in; and the three name their containers
  differently, which `exec` hides.

  `exec` reaches the container through docker on all three, so the words a
  caller quoted stay quoted. Each platform's own exec takes the whole command as
  one string, and `vtysh -c "show ip ospf neighbor"` does not survive that.

  A script reaches only its own lab. containerlab labels a container with the
  lab it belongs to as well as the node it is, and both are matched: another
  lab on the same machine may well have a node of the same name, and teardown
  and collect must not reach into it.

  **`deploy` makes the bridges the topology names**, which containerlab requires
  before it will deploy and which nothing inside the lab can create; `destroy`
  takes the same ones down again. Bringing a lab up therefore needs no step
  outside the script, and the two directions match.

  **`destroy` is also what collects.** A lab that brings files back — every one
  that writes an FRR log does, since the class that makes the log asks for it —
  needs a script for the copying to happen at all.

  **Anything written after `deploy` is passed on** to the platform's own command,
  so a caller can place a lab's management network itself
  (`deploy --network lab7 --ipv4-subnet 172.29.7.0/24`) and run several labs side
  by side. TiNET brings a lab up in two commands and so has nowhere to put an
  extra argument: its script says so rather than dropping it, which is the case
  that costs a caller the most to find out about later.

  Chosen per module, and on wherever a lab needs one: every bundled topology
  that writes a log or names a bridge turns them on, so what the scripts hold is
  checked against `expected/` like everything else — in both scopes, since a
  multi-machine lab gets one script per machine.

  Verified on two machines from the generated files alone: `containerlab.sh
  deploy` on each brings up `topologies/aggregate_crossing` across them, and
  `destroy` brings the logs back and leaves neither containers nor bridges.
- **`--name` on `build`, so one topology can be two labs at once.** A lab's
  containers are named after the lab, so two labs from one topology collide
  unless they are told apart. `--name` replaces the topology's own `name:` for
  that build, and each platform takes the name up the way it namespaces its own
  containers: containerlab in the lab (`clab-lab0-r1`), Kathara and TiNET in the
  device or node name (`lab0_r1`), since those two name a container after the
  node and nothing else.

  The nodes are still `r1`, `r2`, `r3`: their files are written where they
  always were, the configuration inside is untouched, and `exec r1` still
  reaches the node — each entry script knows how its platform spells the name.
  Without `--name` nothing changes at all.

  Renaming at *deploy* time is not offered, and cannot be: containerlab's
  `deploy --name` renames a lab but its `destroy` reads the name from the
  topology file regardless, so a renamed lab cannot be taken down again.
  Naming the lab when it is generated has no such gap.

  Every command that reads a topology takes `--name`, not just `build`:
  `files` has to list what `build` wrote, and `clean` has to delete it.
- **`destroy` in an entry script now takes the lab down in four steps**: the
  lab's `teardown` commands, the files to collect, the platform's own destroy,
  and the lab's own `worker_destroy` commands. A step that fails is named and
  the rest still run: a lab
  left standing because something could not be copied is worse than the missing
  file. The script ends non-zero so that whatever called it knows.

  `collect [<dir>]` runs the collection on its own, and `DOT2NET_COLLECT_DIR`
  sets where the files go.

  A lab is named by the topology, and nothing offers to rename it at deploy
  time. containerlab's `deploy --name` does rename a lab, but its `destroy`
  takes the name from the topology file whatever `--name` says, so a renamed lab
  cannot be taken down again — and the destroy reports success while leaving
  every container running. containerlab's maintainer names this as a known
  limitation. To run one topology more than once at a time, generate it more
  than once under different names.
- **Two labs made from one topology can be deployed on one machine.** A bridge
  reached the machine under the name the topology gave it, so a second lab
  silently joined the first one's bridge — one L2 domain where the topologies
  said two — and whichever was destroyed first took it away from the other.
  Neither said a word.

  Three names had to be told apart, not one: the bridge, the veth reaching it,
  and the lab as containerlab knows it — which for a multi-machine topology was
  the machine's name, so `--name` never reached it. All three now carry the
  run's own name.

  **This also fixes multi-host on ordinary hardware.** A bridge's ports were
  named after the interface at the far end, so a lab whose switch had `eth0`
  asked the machine for a device called `eth0` — the name of the first NIC on
  most machines. Reported by the netroub project, who also worked out that the
  bridge's name alone would not be enough.
- **Names that reach the machine are short and end in a hash**: a bridge is
  `br-a1b2c3` and the veth reaching it `eth0-a1b2c3`, rather than the names the
  topology used. A Linux interface name is at most 15 characters — `IFNAMSIZ` is
  16 and counts the terminator, and an OVS bridge comes with an internal device
  of the same name, so it is subject to the same limit. Fifteen cannot hold a lab
  name, a machine name and a node name, and going over is worse than refused:
  `ovs-vsctl` reports the failure and records the bridge anyway.

  Nobody types these names — a topology writes `{{ .clab_bridge }}` and
  `{{ .opp_clab_host_port }}`, and `dot2net data` says which is which. It is the
  same answer docker gives when it names a container's host-side veth
  `vethXXXXXXX`.

  What is left is eight characters for an interface's own name, so **an interface
  prefix of five leaves room for a thousand ports**. Longer is reported while
  generating.
- **`deploy` stops rather than adopting something that is already there.** A
  bridge left behind by a lab that was not destroyed used to be used as it was
  found; now the script names it and refuses. Deploying builds the same thing
  whatever the machine happens to be holding.
- **Commands a topology runs on the machine, on every platform**: four hooks
  named after the entry script's own commands — `worker_deploy`, `worker_exec`,
  `worker_collect` and `worker_destroy` — run outside the nodes, where `startup`
  and `teardown` run inside them. `worker_deploy` runs before the platform is
  asked to bring the lab up, `worker_destroy` after it has taken it down,
  `worker_collect` beside the files copied out of the nodes, and `worker_exec`
  before a command is carried into one. Anything the lab needs of its machine —
  a bridge to attach to, an interface put into a namespace, a capture started
  and stopped — has somewhere to go.

  They are written on a node class like any other block, and each machine runs
  what its own nodes asked for: a lab spanning machines does not run one
  machine's commands on another. A block that fails is named and the rest still
  run, and `deploy` stops rather than bringing up a lab whose machine is not
  ready.

  Each of the four straddles a command the script gives the platform, and
  **`priority` says which side a block falls on**: the platform's command is the
  origin, below it runs before and above it runs after. Writing no priority gives
  the ordinary case — a module's part is the ground yours stands on, so the bridge
  exists before you attach anything to it, and on `worker_destroy` the order
  reverses so your cleanup runs before the bridge goes.

  What needs saying explicitly is a command that could not have run earlier. A
  veth reaching a bridge is made by the platform as it brings the lab up, so a
  block naming one needs a positive priority; leave it out and **dot2net says so
  while generating** rather than letting it fail on the machine. The same check
  works the other way on `worker_destroy`: naming the veth after the platform has
  taken the lab down is reported too.

  A class a module registers may collect a file of its own — `frrLogFile` offers
  to bring back the log it makes — and a lab generated without an entry script
  simply does not get it. Only what the **topology** asks to collect is an error
  without a script, since nothing else would do it.

  A hook block that no entry script would run — the topology writes one but
  `generate_scripts` is off, or the block belongs to a platform that is writing
  no script — is an error rather than a block that quietly does nothing.

  `worker` because that is what dot2net already calls a machine a lab is
  deployed onto — see the `worker` group class.
- **`collect` on a node class**: files to copy out of a node before the lab is
  destroyed — and the entry script's `destroy` is what does the copying, so a
  topology that collects anything needs one. Each entry is a template, so a path
  that follows a value stays right when the value is changed:

  ```yaml
  nodeclass:
    - name: router
      collect: ["{{ .frr_log_path }}"]
  ```

  Collected files land in `collected/<node>/<the path inside the container>`,
  beside the generated tree rather than in it: what was generated and what came
  back from a run are not the same kind of thing. They are handed to whoever ran
  the lab — `docker cp` keeps the ownership a file had inside the container, and
  the entry script runs under sudo, so without this the collection could not be
  read or deleted without sudo either.

  A module declares what it needs back: `frrLogFile` collects its own log, so a
  topology that never named the file does not have to name it to get it back.

  The entry script does the copying, which is why a topology that collects
  anything needs one. Declaring `collect` with `generate_scripts` off is an
  error naming the node and the file, since the declaration would otherwise sit
  there doing nothing.
- **`teardown` on a node class**: commands to run in a node while it is still
  up, before the lab is destroyed. Dumping state, flushing what a
  program buffers, putting a mounted file's permissions back. Written the same
  way as `startup`, and merged the same way when a module has something to add.
- **`assert` module**: an opt-in module that checks the expectations a topology
  states about itself. A class marked `values: {assert_used: "true"}` must be
  applied to at least one object, or the build fails. It generates no output.
  This closes a hole the golden tests cannot cover: a class that is declared but
  never applied contributes nothing, so regenerating the expected files silently
  freezes its absence — which is how a bundled topology shipped with segment
  classes that never attached.
- **Interfaces are named `eth0`, `eth1`, ... by default** instead of `net0`,
  `net1`. The old prefix followed TiNET's examples; `eth` is what a Linux
  container calls its interfaces and what Kathara requires — it derives the name
  from the index in `lab.conf` and offers no way to change it — so this is the
  one prefix every platform accepts, and a topology can now load all three
  modules at once.

  The exception is containerlab's management network, which keeps `eth0` for
  itself. Turning it back on with
  `module_config.containerlab.management_network` means naming the interfaces
  something else through an interface class `prefix`; dot2net says so while
  generating rather than letting containerlab refuse at deploy time.
- **containerlab nodes are emitted with `network-mode: none` by default.** A lab
  now has only the links its topology describes. The management network containerlab
  attaches by default is convenient — `clab exec`, `clab-*` names — but it is
  also a second path between every pair of nodes, and a reachability test that
  should have failed can pass through it without anyone noticing. TiNET runs its
  nodes with `--net none` and Kathara gives them no management network either,
  so this also brings the three into line.

  It frees `eth0` as well: containerlab refuses a data interface by that name
  while the management network is attached, which is what kept a topology from
  loading the Kathara module alongside the other two.
- **A shared segment reaching across machines is replaced with one bridge per
  machine**, and the bridges are linked. A shared segment is drawn as a node the
  platform provides rather than deploys, and such a node stands on one machine;
  left alone, every member on another machine needs a link that leaves its own.
  So a segment with n members over there costs n links leaving a machine — and
  each one costs a VLAN from a finite pool. With the replacement it costs one
  per pair of machines, whatever n is.

  The author draws the segment they mean, once. `topologies/aggregate_crossing`
  shows the difference: four routers on one segment, two per machine, cost four
  crossings without it and one with it. The addresses do not move either way —
  bridges carry none, so a search for a segment passes through them and all four
  routers stay in one subnet.

  A link between two such nodes is not counted as a member: it is the link
  between two sides of a segment that is already split, whether dot2net made it
  or the author wrote it out. `topologies/ospf_multihost`, which writes the split
  by hand, is unchanged.

  `global.aggregate_crossing_links: false` turns it off, for a platform that
  stretches a segment across machines itself or an author who wants to draw the
  split. It defaults to on.
- **Wired interfaces are numbered first.** Within one prefix, the numbers run
  without a gap over the interfaces the platform lays. Kathara names an interface
  after its position in `lab.conf`, so a number spent on an interface that gets
  no line there used to leave a hole it refuses to start on.
- **`deploy` on node classes**: names what the platform puts in place for a node
  — `container` (a container of its own), `platform` (a facility the platform
  provides itself) or `none` (nothing is deployed; the node carries parameters
  only). Unknown values are rejected.

  A `platform` node is emitted with its `kind` alone — no image, no bind mounts,
  no commands — and is exempt from the image requirement. This is what lets a hub
  of N members replace N² point-to-point links. dot2net never reads or writes the
  `kind` value itself: whether it is `bridge`, `ovs-bridge` or something
  containerlab adds later is passed through from the user untouched, so a new
  kind needs no change here. TiNET realizes the same node as an OVS bridge of its
  own: the node moves from `nodes:` to a `switches:` section, and the interfaces
  facing it attach by name (`type: bridge, args: <switch>`) instead of naming a
  peer interface. The section is omitted entirely when a topology has no switch.
  An OVS *container* needs none of this — it is an ordinary node, as
  `example/switching` already shows with a Linux bridge.

  Omitting `deploy` is not a claim that the node is a container: it leaves the
  choice to the other classes, and only a node no class speaks for falls back to
  `container`. That is why a class that says nothing never collides with one that
  asks for `platform`. Claims are weighed by class tier, so a default stated by
  the base class of `class_policy` is overridden by a class the user named on the
  node — which is how a topology sets its own default without a dedicated global
  setting.

  `virtual: false` still claims nothing, since a boolean cannot tell "false" from
  "unset"; `deploy: container` is how to say it out loud and override another
  class. Only the field name is reserved, so a class may still be *called*
  `switch` and mean an ordinary container, as eight of the bundled examples do.
- **`deploy` on interfaces and connections**, with the values that kind of object
  can take: `link` (wiring the platform lays, or an end of it), `logical` (a
  device or a link the generated configuration builds — a bridge, a dummy, a VRF,
  a GRE or VXLAN tunnel), or `none`.

  This completes an axis that only nodes had. It asks one question of every
  object — what is this materialised as — and the values differ by kind because
  what an object can be differs: a node has two ways of being put in place by the
  platform and no way of being built from inside itself, while wiring has one
  platform form and can be built by the configuration instead. A group has no
  `deploy` at all, since nothing is ever put in place for one.

  An interface takes the form of the connection it sits on, so the usual case
  needs nothing written; a class says it for an interface that has no connection
  to take it from. A connection reaching a node nobody deploys is not
  materialised either, and neither is an interface on it — which is where the
  reach that `virtual` used to have on a node has gone.

  `required_link` templates are written only where the connection is a `link`,
  which is what keeps a platform from being told to wire a tunnel. The check now
  applies to connection-scoped templates as well as interface-scoped ones.
- **`use:` on class definitions**: a class may name other classes of the same
  type that an object carrying it also carries. It attaches labels only — no
  field is merged or overridden — so composition follows the ordinary
  multi-class rules. Cycles terminate; naming an undefined class is an error.

  This is what lets a module offer a ready-made class that a topology opts into,
  without naming that class in the topology: putting it in the DOT would tie the
  file to one platform, since the class exists only while that module is loaded.
  A used class keeps the tier of wherever it was defined rather than the tier of
  the class that named it, so a module's defaults still lose to the user's own
  values instead of clashing with them.
- **`worker` group class**: marks a placement unit, a machine that containers
  are deployed onto, as opposed to a group that exists to share parameters. A
  node belongs to several groups at once, so the two uses need telling apart.
  dot2net owns the word rather than letting each platform module pick its own,
  because fourteen of the bundled examples emit both `topo.yaml` and `spec.yaml`
  from one topology and a name like `clabHost` in the DOT file would tie it to
  containerlab.
- **`boundary_crossing_connection_class` on a group class**: names a connection class attached to
  every connection that leaves a group of that class. Whether the two ends sit
  in the same group follows from the topology, so annotating each edge would
  state twice what is already written once, and the two can disagree. The label
  goes on at the module tier, so a class written on the edge outranks it. The
  feature knows nothing about hosts: setting it on an `as` class marks the eBGP
  sessions just as setting it on the worker class marks the links that leave a
  machine.
- **Write a node's `startup` once and every platform runs it, including what a
  module adds to it.** Which names cross between a module and a topology is now a
  short list rather than a growing one, and `use:` is the only way in:

  - the **class names and value names a module publishes** — you write
    them in `use:` and `values:`
  - the **hook names dot2net itself owns** — you write them in
    `config: - name:`. There is one so far, `startup`, and a module reads it and
    puts it where its own platform expects it

  A module's own block names are its business. Naming one from a topology would be
  reaching into a module's insides, and the ways the two can talk to each other
  would multiply with every module and every feature until nobody could say what
  they are.

  What this makes possible: a module class pulled in with `use:` can define a
  template under a hook name, and what it has to do is merged ahead of what the
  topology asked for there. Two classes of the topology's own naming one hook is
  still rejected — nothing would say which of them wins.

  ```yaml
  nodeclass:
    - name: router
      use: [frrLogFile]     # the module adds its commands to this node's startup
      config:
        - name: startup
          template: ["whatever the topology wants, running after"]
  ```
- **`provide` on a file definition**: says how the file reaches its node's
  container, `mount` (the default) or `copy`. Neither is the better one, and
  which to use follows from what the file is for:

  ```yaml
  file:
    - name: frr.conf
      path: /etc/frr/frr.conf   # provide: mount, and must be - FRR reads it while booting
    - name: motd
      path: /etc/motd
      provide: copy             # the container may rewrite it; the generated file stays as generated
  ```

  A **mounted** file is the generated file itself, shown to the container. It is
  there before the container's first process runs, which is the only way to
  reach software that reads its configuration while booting. The other side of
  that: what the container writes reaches the generated file, and a container's
  own startup can take ownership of it — measured on Kathara, where a writable
  mount left `/etc/frr` owned by the container's user and the generated files out
  of the author's reach.

  A **copied** file is placed once the container is up, from a read-only staging
  directory (`r1/staging/etc/motd`, mounted at `/staging`). The container gets
  its own copy: it may rewrite it, and nothing comes back. It cannot serve a
  file read while booting, on any platform — containerlab's `exec:`, TiNET's
  `cmds:` and Kathara's `<device>.startup` all run after the container has
  started.

  Nothing falls back silently. A combination a platform cannot honour is
  reported: a file to be mounted from a directory Kathara was not told it owns,
  a copy landing inside a mounted directory (the mount is read only, so the copy
  would fail at deploy time — on Kathara without a word), a `mount_dirs` entry
  that no file is generated into.

  `dot2net files -v` lists the delivery beside each file. The plain listing
  stays a list of paths, since `dot2net clean` reads it.
- **`executable` on a file definition** writes the file with the executable bit
  set. The generated entry scripts use it: a script that
  has to be `chmod`'ed before it works is one that will be run wrong once.
- **`raw` on a config entry**: hands a source file through as read instead of
  reading it as a template. For a file that is material rather than a template —
  one dot2net has nothing to fill in, and that may carry `{{` of its own meant
  for whoever reads it later. Until now every `sourcefile:` was expanded, so
  such a file either failed to parse or, worse, had an action quietly filled in
  because its name happened to match a parameter. The bundled examples use it
  for the FRR `daemons` and `vtysh.conf` files they ship.
- **`delimiters` on a config entry**: replaces `{{` and `}}` for that template
  alone, e.g. `delimiters: ["[[", "]]"]`. Generating a file that is itself a
  template for another tool — an Ansible playbook, a TENTOU `infra.yml` — would
  otherwise mean escaping every one of its actions, and an action naming a
  parameter dot2net knows would be swallowed without a word. With different
  marks the downstream syntax passes through and dot2net's own values are still
  filled in, in the same file.
- **`values:` on segment classes**: `segmentclass` accepts a `values` map like the
  other class types, so a segment can carry static attributes
  (`values: {kind: ovs-bridge}`). Conflicting values from two classes of the same
  tier are rejected, as they are for the other class types.

- **Group-scope file output**: a `groupclass` can now own a `file:` template, and
  `file` definitions accept `scope: group`. The file is written into the group's
  own directory (`cluster_h1/host.yaml`), mirroring how node-scope files land in
  the node's directory; `output: root` puts it at the output root instead, where
  `name_prefix` / `name_suffix` are needed to keep the groups apart.
- **Group child aggregation**: group-scope templates can reference
  `{{ .nodes_<name> }}` and `{{ .connections_<name> }}` and receive only the
  group's own members. A connection is included only when both of its endpoints
  are inside the group, so a link leaving the group appears in no group file.
- **`xlabel` is read on edges and subgraphs**, not only on nodes. It was silently
  ignored there, so `sw1 -> sw2 [xlabel="trunk"]` attached no connection class —
  which is what `example/value_class_basic` had been doing. For a subgraph,
  `xlabel` is also the way to give a cluster a decorative title without it being
  taken as a group class (`label` still doubles as both). Node `label` remains
  excluded on purpose: it collides with the record-shape node syntax.
- **`module_config`**: a section per module, for settings that belong to one
  platform rather than to the topology.

  ```yaml
  module_config:
    containerlab:
      management_network: true
  ```

  Kept apart from `global:`, which holds what every platform shares. What a
  module offers is its own - containerlab's management network and Kathara's
  bridged devices sound alike and are not the same thing, so a shared key would
  be wrong. A section naming a module the topology does not load is rejected,
  since it does nothing and is nearly always a typo.

  `containerlab.management_network` is the first setting to live there: it puts
  the management network back for a topology that wants `clab exec` and the
  `clab-*` names.
- **`global.output_group_class`**: names the group class that splits the output
  directory. Every node of such a group has its files written below that group's
  directory, so a host packs as a single directory:
  `clabhost1/{topo.yaml, r1/frr.conf, r2/frr.conf}`. Network-scope files belong
  to no group and stay at the output root. A node in two groups of that class is
  reported as an error rather than silently assigned to one. Unset (the default)
  keeps the previous flat layout.

- **`global.split_module_output: true`** puts each module's own files in a
  directory named after it — `containerlab/topo.yaml`, `tinet/spec.yaml` —
  while the files the topology defines stay where they are, since more than one
  platform may read them. Bind and mount paths follow. Off by default; it is for
  a lab whose output is large enough that the platforms get in each other's way.

  Kathara is not split: its lab *is* the directory, holding `lab.conf` and every
  `<device>.startup`, and those startup files are written by the topology.
- **`frrLogFile`**: a class from the FRR module that makes the log file and names
  it to a running FRR. FRR cannot do it itself — its daemons run as the frr user
  and `/var/log` belongs to root, so a log file named in a configuration read at
  boot is one FRR reports it cannot open, and mounting the file in was the old
  answer. That answer does not survive Kathara, which mounts directories and
  would have to replace `/var/log` wholesale. Doing it from `startup` costs the
  messages FRR logged before it runs, and nothing after.

  `topologies/ospf_topo1`, `ospf6_topo1`, `rip_topo1`, `bgp_features` and
  `bgp_evpn_vxlan_topo1` use it and no longer generate a log file of their own.

  **The log now lives only inside the node.** It used to be a file in the
  generated directory that FRR wrote through a bind mount, so anything watching
  that directory saw it grow; there is nothing there to watch any more. Use
  `collect` to copy it out — see
  [Entry point scripts](https://github.com/cpflat/dot2net/wiki/Command-Reference#entry-point-scripts).
- **Ready-made bridge setup classes (containerlab)**: `clabOvsBridgeSetup` and
  `clabLinuxBridgeSetup` carry the commands that create and delete the bridge
  containerlab requires to exist before deploy. A topology opts in with
  `use: [clabOvsBridgeSetup]`; both write `worker_deploy` and `worker_destroy`,
  so the entry script makes the bridges before deploying and takes them down
  after destroying. Deploying a multi-machine lab is one step again — nothing
  has to be run by hand beforehand, and nothing is left behind afterwards.

  They are never applied automatically. Which command creates a bridge is not
  decided by the kind — the same `ovs-bridge` may be provisioned by Ansible,
  need sudo, or live in another OVS database — so a topology that does it
  differently names neither class and writes its own `worker_deploy`.
- **An unknown or duplicate key in the config file is now an error.** A key the
  config does not know used to be dropped without a word, which looks exactly
  like a setting that had no effect: `example/address_reservation` wrote
  `management_layer` where the key is `mgmt_layer`, and spent a year with its
  management network quietly switched off. A duplicate key was the same kind of
  quiet loss, with the later value winning. The message names the key and the
  line. Settings inside `module_config` are checked the same way.

  All bundled topologies already pass; a config that does not will name what to
  fix.
- **Two file definitions writing the same file is now an error.** A file's
  content is ordered by the config templates behind it, and two definitions
  landing on one path have no such order: the one written last won and the
  other's content was gone without a word. The names of a definition and of the
  file it writes are separate things — `name_prefix`/`name_suffix` build the
  filename — so `startup` and `kathara_startup` were different definitions both
  writing `r1.startup`. The message names both definitions and the file.

- **A config entry naming both `template` and `sourcefile` is now rejected.**
  Which came first was decided in the code and written down nowhere; nothing
  used it. Write two entries and order them with `blocks:`.
- **A class a module attaches now loses to one you wrote, instead of colliding
  with it** (`LabelOwner.AddModuleClassLabels`, for module authors). A module
  classifying objects through the `ObjectClassifier` hook had only
  `AddClassLabels`, which files labels as user-written, contrary to the rule that
  module-provided classes are the weakest.

### Changed

- **A node's files are laid out by the path they take inside the container.** A
  file declared as `path: /etc/frr/frr.conf` is written to
  `r1/etc/frr/frr.conf`, where it used to be written to `r1/frr.conf` and the
  `path` was used only as the mount target. **This changes where generated files
  appear**, so anything reading them by path — netroub, for one — has to follow.

  Kathara is why: it delivers a device's files by copying the device's own
  directory into the device, so the directory has to mirror the container's
  filesystem for the files to land anywhere useful. containerlab and TiNET only
  see a different source path in their mounts.

- **`topologies/ospf_topo1`, `ospf6_topo1` and `rip_topo1` ran no routing software.**
  Every node was `nicolaka/netshoot`, which has no FRR, while the topologies
  generated `zebra.conf`, `ospfd.conf` and the rest for it to read. They had
  been that way since a golden test gave every node an image; before that they
  named none. Their routers now run FRR, and all three do what their names say
  when deployed: OSPF and OSPFv3 reach Full, RIP learns its neighbours'
  networks. The switches stay on netshoot — they are bridges made with
  `ip link`, which FRR's image cannot do.
- **`example/three_platforms` is gone**, and each root's `readme.md` says what
  its topologies are for: `topologies/readme.md` which of them generate for
  Kathara, why some cannot, and where `basic_mpls`, `large_clos` and
  `large_ring` went when v0.4.0 changed the format under them;
  `example/readme.md` what each notation demonstration shows.

### Fixed

- **IP policies ignored the class tiers**: a policy was applied as each class
  was visited, and classes are visited strongest first, so the last write won —
  the *weakest* class silently took the layer. A base class could therefore
  override the policy a node's own class had set. Policies now resolve like
  values: the stronger class wins, and two classes of the same tier naming
  different policies for one layer is an error. **Two classes named together in
  a DOT label that set different policies for the same layer are now rejected**
  instead of resolving arbitrarily.
- **A class pulled in with `use:` lost to the base class**, or failed the build
  outright. Class labels are not listed in order of precedence — `use:` appends
  the classes it reaches after everything else, keeping the tier they were
  defined at — but four places resolved attributes by walking that list and
  keeping the first value they saw. A base class stating `values: {mtu: 9000}`
  therefore beat a user class that used a class stating `1500`, and where the
  clash was checked the build stopped instead, calling two classes "the same
  precedence" when one outranked the other. Values, policies, prefixes and
  `mgmt_interfaceclass` now resolve by tier whatever order the classes come in.
  Connection and segment names follow the prefix that was resolved rather than
  re-deriving their own, which could disagree with it.
- **The TiNET `switches:` list ran its entries together on one line.** It
  borrowed the format that renders the inline interfaces list, where `", "` is
  right. No example had two switches before, so it never showed.
- **TiNET bind mount sources were relative, and Docker refuses them** — it reads
  a relative source as a volume name, so a generated `spec.yaml` could not be
  deployed as written. The paths have been relative since the module refactor
  that moved tinet out of `pkg/`, which dropped the `filepath.Abs` the earlier
  implementation used. Resolving at build time would bake the generating machine
  into the output, so the source now carries a literal `$PWD`, expanded by the
  shell that `tinet up` output is piped to — which runs where the spec file sits.
- **`dot2net files` omitted every per-machine `topo.yaml`, and `dot2net clean`
  left them behind.** The file-list pipeline stopped before classification, and
  a topology file scoped to a worker group is assigned there rather than when
  the module is loaded. The single-machine case hid it: that topology file is
  network-scoped and registered at load time. The pipeline now runs the same
  classification a build does, boundary connections included.
- **A node in two `worker` groups built without complaint and wrote
  contradictory output**: it was listed in the topology file of both machines,
  and the links inside one machine were marked as leaving it, because its two
  endpoints then belong to different sets of worker groups. Nested or
  overlapping worker subgraphs are easy to write. The invariant existed — a
  placement unit holds each node once — but only the output directory enforced
  it, and only when the output is split by group and the node writes a file of
  its own. It is now checked on the topology itself. Groups that merely share
  parameters are unaffected: an AS spanning two machines nests and overlaps as
  before.
- **Two classes on one object defining the same config template name** are now
  reported when the classes are resolved, naming both classes. The clash used to
  surface later as a duplicated namespace parameter that named neither — hard to
  trace when one of them arrived through `use:`.
- **`example/address_reservation`'s management layer had been disabled since
  2025-09-11**: it wrote `management_layer:` where the key is `mgmt_layer:`, and
  an unknown key is dropped in silence, so no management address was ever
  assigned. The key is fixed and the address now reaches the generated config,
  so the example verifies the feature it declares instead of merely naming it.
- **`clean` left the group directories behind**: it only considered the
  immediate parent of each generated file, so with group-scope output the empty
  `host1/` remained after its contents were removed. Every ancestor is now
  considered, deepest first.
- **Bind mounts pointed at the wrong path under `output_group_class`**: the
  containerlab `binds:` and TiNET `mounts:` entries were built as
  `<node>/<file>` and ignored the group directory the file is actually written
  into, so they named a file that does not exist. All three places that need
  that path — writing the file, listing it, and mounting it — now go through
  `Node.OutputPath`.
- **Segment aggregation mixed the layers**: the parameter a parent aggregates a
  segment's config into is now `segments_<layer>_<name>` instead of
  `segments_<name>`, matching what block references (`blocks.before` /
  `blocks.after`) already expected and what neighbor aggregation already did.
  A parent sees the segments of every layer in one pass, so without the layer
  two layers using the same config name were merged silently. **This renames the
  parameter**: no bundled topology or module referenced it, but a topology
  outside this repository did, and the failure comes late enough to look like
  something else. It is listed under Breaking changes for that reason.
- **An empty config block was formatted into a blank command.** A block that
  rendered to nothing still had the line prefix applied, so containerlab found an
  empty entry in `exec:` and would have tried to run it. The merge phase had
  always dropped empty blocks; the formatting step now does the same.
- **`dot2net files` listed the configuration of a node whose configuration is
  withheld**, which the build does not write. The list and the build now agree.
- **`NetworkSegment.Layer` was never assigned**, so it read as empty everywhere
  (debug messages, and now the aggregation parameter name).
- **Missing aggregation parameters**: a named child template now contributes its
  `{{ .nodes_... }}` / `{{ .connections_... }}` parameter as an empty value even
  when no child object produced output, instead of the parameter being absent
  and the referring template failing. A name that matches no defined template is
  still an error, so typos are still caught.
- **`Group.Nodes` was never populated**, leaving groups with no children and no
  ordering constraint against their nodes. Group membership is now recorded in
  both directions.

## [0.7.3] - 2026-07-30

### Changed

- **DOT parsing dependency**: replaced the forked gographviz
  (`cpflat/gographviz` via a `replace` directive) with the standalone
  [dotlike](https://github.com/cpflat/dotlike) v0.0.1 library.
  `DiagramFromDotFile` now uses dotlike's one-shot `Parse` (collapsing the
  previous `Parse`→`NewGraph`→`Analyse` sequence). No behavior change; output
  remains byte-stable.

## [0.7.2] - 2026-07-16

### Fixed

Bug fixes from a systematic code review (correctness, determinism, and robustness):

- **Deterministic output**: sort links, generated file lists, group creation, and parameter-rule processing so builds are byte-stable across runs (previously map / edge iteration order could vary)
- **`param_format` expansion**: `param_format` values are now expanded as `text/template` (e.g. `VLAN{{ .value }}`) instead of being assigned as the literal template string
- **Multiple NodeClass matching**: `NodeClassCheck` / `NeighborNodeClassCheck` no longer drop the plural `nodes:` / `neighbor_nodes:` list (a `copy` into a zero-length slice was a no-op)
- **Address calculation**: `getitem` now carries into the most significant byte and handles prefix lengths ≤ 8 (previously it could ignore the index or drop the top-byte carry); `prefixToIndex` uses big-integer math (correct for IPv6-sized differences, no per-byte borrow bug), and `reserveAddr` skips addresses outside the pool instead of recording a bogus index; pool enumeration works in log order (no `2^n` overflow) and is bounded by the new `max_address_count` setting
- **Bind mount source path**: tinet / containerlab bind mounts use the actual generated filename (`GetFileName`) instead of the file-definition id, fixing paths when `name_prefix` / `name_suffix` is set
- **Errors instead of panics**: return errors (rather than panicking) on missing dot-file arguments, malformed DOT input, ambiguous edges during diagram merge, out-of-range member / neighbor references, and insufficient `file` parameter candidates; guard the class type assertions in connection / segment auto-naming so a mixed-type class is skipped instead of crashing
- **Configuration validation**: `LoadConfig` now rejects duplicate class names (node/interface/connection/group/segment) and duplicate `param_rule` names instead of silently keeping the last definition; a `param_rule` integer source with `max < min`, and an empty (`start == end`) or inverted (`start > end`) `sequence` source, are now reported as configuration errors instead of being silently accepted (the sequence case previously produced 10 entries). `max: 0` continues to mean "no upper bound"
- **Error propagation**: surface previously-discarded errors from place-label checks, management-interface labels, group labels, and the relative-namespace chain (unknown PlaceLabel referenced by a MetaValueLabel); fix `%w` error wrapping so failures carry their cause
- **Class label validation**: an undefined class label is now reported (or skipped, per `ignore_undefined_class`) consistently across node/interface/connection/group; corrected mislabeled "interfaceclass" errors for connection/group classes
- **Diagram merge**: merging diagrams no longer drops node groups that exist only in the second diagram
- **Containerlab endpoints**: no longer emit a dangling `endpoints:` line when a node has no non-virtual connections
- **Visual output**: guard against nil node / interface references and wrap the underlying error when an interface lacks an IP address
- **`suggestAlternativeName`**: suggest a genuinely different name for the `node_` / `group_` reserved prefixes (previously returned the same name)

### Added
- **`max_address_count` global setting**: caps how many addresses/prefixes are enumerated when an address pool is expanded fully (default 65536), preventing oversized (e.g. IPv6) pools from exhausting memory
- **`ignore_undefined_class` global setting**: when `true`, class labels that match no defined class are skipped instead of causing an error (e.g. subgraph labels used only for display); default `false`
- Regression tests for `getitem` / `prefixToIndex` address arithmetic and enumeration caps, multiple-NodeClass matching, and `param_format` expansion
- **Expanded test suite** covering previously-untested core code (no behavior change):
  - `pkg/types/object.go`: class conflict detection, relative-namespace prefix composition, group parameter precedence, and construction boundaries (isolated node, self-loop, multi-edge)
  - `pkg/model/dependency.go`: topological sort and cycle detection (self-loop, multi-node cycles, cycle-path recovery, missing/errored dependencies) exercised directly
  - `pkg/visual`: `abbreviateIPAddress` / `getInterfaceAddress` plus structural checks of `GraphToDot` (parseable DOT) and `GetDataJSON`
  - CLI: end-to-end `clean` safety test (deletes only generated files/emptied directories, never user files) and `--dry-run`
  - Module output: structural validation of generated containerlab / tinet YAML (node kind/image, links, interfaces) instead of filename-suffix matching
  - Determinism: builds each representative scenario twice and asserts byte-identical output
  - Failure paths: malformed DOT / YAML and undefined-class references (strict vs. `ignore_undefined_class`)

## [0.7.1] - 2026-02-05

### Fixed
- **GroupClass ConfigTemplate initialization**: Fixed bug where `groupclass` config templates were not initialized in `LoadTemplates()`, causing `{{ .groups_template_name }}` references to fail
- **Group parameter namespace**: Fixed bug where Group's own parameters were not copied to `relativeParams` in `BuildRelativeNameSpace()`, causing template variables like `{{ .hostname }}` to be missing

### Changed
- **Tutorial updated**: Synchronized tutorial with example/ospf_simple
  - DOT: Changed `class=` to `xlabel=` (recommended for visualization)
  - YAML: Added `blocks.after` for frr.conf (v0.6.0 feature)
- **example/bgp_features**: Refactored to use `blocks.after` with `self_` prefix for cleaner BGP config template composition, eliminating empty lines when iBGP/eBGP is not present

### Removed
- **Legacy primary flag references**: Removed all remaining `primary: true` from tutorial, tests, and all example scenarios, and deleted commented-out primary-related code from `pkg/model/model.go` (primary flag was deprecated in v0.5.0)

## [0.7.0] - 2026-01-03

### Added
- **FileDefinition output control**: New fields for flexible file output location
  - `output` field: `root` outputs to lab directory root, default outputs to node subdirectory
  - `name_prefix` field: Prepend prefix to output filename (e.g., `init_` → `init_r1`)
  - `name_suffix` field: Append suffix to output filename (e.g., `.startup` → `r1.startup`)
  - Enables Kathara-style startup files: `r1.startup`, `r2.startup` in root directory
- **ConfigTemplate required_params**: Conditional config block generation based on parameter existence
  - `required_params` field: List of parameters that must exist for block to be generated
  - If any required parameter is missing, entire block is skipped
  - Useful for optional parameters like `mem`, `cpus`, `sysctl` in Kathara/Containerlab configs
- **Value class**: Virtual objects for multi-value parameter generation
  - New `mode: attach` for param_rules - attaches multiple Values to single objects
  - `source` field for Value generation: `range`, `sequence`, `list`, `file` types
  - `generator` field for module-provided Value generation (e.g., `clab.filemounts`)
  - `config_templates` in param_rules for Value-specific formatting
  - Template reference syntax: `{{ .values_xxx }}` for formatted Value output
- **ParameterGenerator interface**: Modules can now generate Value parameter lists dynamically
- **Module bind mounts via Value class**: Containerlab and TiNet modules now use Value class for file mounts
  - `clab.filemounts` generator for containerlab bind mounts
  - `tinet.filemounts` generator for tinet bind mounts
- **AddParameterRule method**: Config can now have param_rules added dynamically by modules
- **File source format support**: YAML/JSON/CSV file parsing for Value generation (auto-detected by extension)
- **Reserved parameter name check**: Validation for param_rule names against reserved prefixes
  - Prevents collision with internal prefixes: `node_`, `conn_`, `values_`, etc.
  - Reserved names: `name`
  - Check functions: `ReservedPrefixes()`, `ReservedNames()`, `CheckReservedParamName()`
- **Windows CI/CD support**: Added Windows to GitHub Actions workflow
  - `.gitattributes` for consistent LF line endings across platforms
  - Windows test environment (`windows-latest`)
  - Windows binary release (`dot2net-windows-amd64.exe`)

### Changed
- **Module templates updated**:
  - Containerlab: `._clab_bindMounts` → `{{ .values_clab_bind_entry }}`
  - TiNet: `._tn_bindMounts` → `{{ .values_tinet_bind_entry }}`
- **Containerlab template refactoring**: Replaced if-statements with `required_params`
  - Split `topo.yaml.node_clab_topo` into separate binds/exec templates
  - Uses `required_params` for conditional section output (no if-statements in templates)
- **Internal refactoring**: `addressedObject` renamed to `layerAwareObject` for clarity
  - Reflects actual purpose: Layer (IP address space) awareness and policy management
  - Consistent with `AwareLayer()` method naming

### Removed
- **FormatStyle legacy fields** (BREAKING CHANGE): Removed deprecated fields from v0.6.0
  - `lineprefix` → use `format_lineprefix`
  - `linesuffix` → use `format_linesuffix`
  - `lineseparator` → use `format_lineseparator`
  - `blockprefix` → use `format_blockprefix`
  - `blocksuffix` → use `format_blocksuffix`
  - `blockseparator` → use `merge_blockseparator`
- **Unused constants and files**:
  - `NumberPrefixOppositeHeader` constant (duplicate of `NumberPrefixOppositeInterface`)
  - `pkg/model/namespace.go` (functions moved to appropriate files)
  - `pkg/model/model_test.go` and test fixtures (covered by `internal/test`)
  - Legacy module constants (`DefaultNamespaceFormatName`, `DefaultAssemblyFormatName`)
- **Empty directories**: `pkg/clab/`, `pkg/tinet/` (modules moved to `mod/`)

### Fixed
- **Example scenario**: `vlan_multihost` param_rule `conn_id` renamed to `connection_id` to avoid reserved prefix collision

### Deprecated
- Legacy bind mount parameters (`_clab_bindMounts`, `_tn_bindMounts`) replaced by Value class

## [0.6.2] - 2025-12-31

### Added
- **GitHub Actions**: Automated release workflow for multi-platform binary distribution
  - Test environments: Linux, macOS (ARM/Intel)
  - Build targets: linux-amd64, linux-arm64, darwin-amd64, darwin-arm64
  - Trigger: Tag push with `v*.*.*` format

## [0.6.1] - 2025-12-05

### Fixed
- **Empty bindmounts**: Fixed containerlab module generating empty bind mounts for nodes without mounted files

## [0.6.0] - 2025-12-03

### Added
- **FormatStyle**: New `FormatStyle` structure replacing `FileFormat` with clearer phase separation
- **Format Phase fields**: `format_lineprefix`, `format_linesuffix`, `format_lineseparator`, `format_blockprefix`, `format_blocksuffix`
- **Merge Phase fields**: `merge_blockseparator`, `merge_resultprefix`, `merge_resultsuffix`
- Legacy field fallback mechanism for v0.6.x backward compatibility
- **FileGenerator interface**: NetworkModel and Node implement FilesToGenerate() to determine generated files based on class labels
- **Test verification**: Added files command output verification in example tests

### Changed
- **FileFormat → FormatStyle**: Renamed structure to align with YAML `format:` section naming
- **Phase separation**: Clear distinction between Format Phase (block generation) and Merge Phase (block merging)
- **Module updates**: All 4 modules (builtin, frr, containerlab, tinet) updated to use FormatStyle
- **TiNET module**: Migrated `tn_config` template to use `blocks.after` instead of direct embedding
- **Example scenarios**: Migrated 4 scenarios (switching, ospf_simple, param_share, vlan_multihost) to use `blocks.after`
- **File listing**: ListGeneratedFiles and module file mounts now honor class labels (only mount files that nodes actually generate)
- **BuildNetworkModelForFileList**: New lightweight version for file listing that skips IP assignment and parameter generation

### Fixed
- **Double formatting issue**: Removed duplicate format application in merge phase
- **childConfigs bug**: Fixed issue where unformatted configs were stored in childConfigs
- **Merge optimization**: Reduced merge operations from 2 to 1 in `processConfigTemplateWithBlocks()`
- **Invalid file mounts**: Fixed bug where nodes mounted files they don't generate (e.g., r3/bgpd.conf when r3 has no bgp class)

### Deprecated
- Legacy fields in `FileFormat` (will be removed in v0.7.0):
  - `lineprefix` → use `format_lineprefix`
  - `linesuffix` → use `format_linesuffix`
  - `lineseparator` → use `format_lineseparator`
  - `blockprefix` → use `format_blockprefix`
  - `blocksuffix` → use `format_blocksuffix`
  - `blockseparator` → use `merge_blockseparator`

### Documentation
- Added comprehensive release notes: `doc/active/V0.6.0_RELEASE_NOTES.md`
- Archived completed planning documents to `doc/archive/completed/`
- Added future improvement TODO: `doc/active/TEMPLATE_CONDITIONAL_BLOCKS_TODO.md`
- Updated CLAUDE.md with v0.6.0 changes

## [0.5.1] - 2025-09-17

### Fixed
- Bug fixes in connection/segment parameter assignment

## [0.5.0] - 2025-09-17

### Changed
- **Eliminated primary flag**: Removed primary flag usage from address assignment
- **Config block workflow**: Changed config block generation workflow
- **Dependency graph**: Use generalized dependency graph implementation for reorderConfigTemplates

### Added
- **Clean subcommand**: Added `clean` subcommand to remove generated files and empty directories
- **Golden tests**: Added golden test for example scenarios
- **Tutorial**: Added tutorial documentation

### Fixed
- Bug fixes in virtual objects and layers in interface classes

## Earlier Versions

For earlier version history, see git commit log.

[0.8.0]: https://github.com/cpflat/dot2net/compare/v0.7.3...v0.8.0
[0.7.3]: https://github.com/cpflat/dot2net/compare/v0.7.2...v0.7.3
[0.7.2]: https://github.com/cpflat/dot2net/compare/v0.7.1...v0.7.2
[0.7.1]: https://github.com/cpflat/dot2net/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/cpflat/dot2net/compare/v0.6.2...v0.7.0
[0.6.2]: https://github.com/cpflat/dot2net/compare/v0.6.1...v0.6.2
[0.6.1]: https://github.com/cpflat/dot2net/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/cpflat/dot2net/compare/v0.5.1...v0.6.0
[0.5.1]: https://github.com/cpflat/dot2net/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/cpflat/dot2net/releases/tag/v0.5.0
