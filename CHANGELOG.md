# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **One TiNET spec file per machine.** When the scenario declares `worker` groups
  the spec file becomes group-scoped, the same way the containerlab topology
  file does: each machine gets its own nodes and the links it can wire itself,
  and the link that leaves a machine appears in neither. Mount paths become
  relative to the machine's directory. Scenarios that declare no placement unit
  keep the single network-scoped file. Verified on two hosts: the same DOT and
  YAML bring up an OSPF adjacency across the machine boundary on TiNET as well
  as on containerlab.
- **`example/ospf_multihost`**: `example/ospf_simple` placed on two machines —
  the same OSPF configuration, split across a machine boundary. This is the
  scenario to copy when writing a multi-host topology; `example/vlan_multihost`
  demonstrates the machinery (`worker`, `boundary_class`, `deploy`, `use:`) and
  configures no routing. Verified on two hosts: the OSPF adjacency between the
  border routers comes up across the boundary, each machine learns the other's
  subnets, and traffic is routed between them.
- **`xlabel` is read on edges and subgraphs**, not only on nodes. It was silently
  ignored there, so `sw1 -> sw2 [xlabel="trunk"]` attached no connection class —
  which is what `example/value_class_basic` had been doing. For a subgraph,
  `xlabel` is also the way to give a cluster a decorative title without it being
  taken as a group class (`label` still doubles as both). Node `label` remains
  excluded on purpose: it collides with the record-shape node syntax.
- **Ready-made bridge setup classes (containerlab)**: `clabOvsBridgeSetup` and
  `clabLinuxBridgeSetup` carry the command that creates the bridge containerlab
  requires to exist before deploy. A scenario opts in with
  `use: [clabOvsBridgeSetup]`; both define a `clab_bridge_setup` template, so
  `{{ .nodes_clab_bridge_setup }}` collects one line per switch node.

  They are never applied automatically. Which command creates a bridge is not
  decided by the kind — the same `ovs-bridge` may be provisioned by Ansible,
  need sudo, or live in another OVS database — so a scenario that does it
  differently names neither class and writes its own `clab_bridge_setup`.
- **`use:` on class definitions**: a class may name other classes of the same
  type that an object carrying it also carries. It attaches labels only — no
  field is merged or overridden — so composition follows the ordinary
  multi-class rules. Cycles terminate; naming an undefined class is an error.

  This is what lets a module offer a ready-made class that a scenario opts into,
  without naming that class in the topology: putting it in the DOT would tie the
  file to one platform, since the class exists only while that module is loaded.
  A used class keeps the tier of wherever it was defined rather than the tier of
  the class that named it, so a module's defaults still lose to the user's own
  values instead of clashing with them.
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
  node — which is how a scenario sets its own default without a dedicated global
  setting.

  `virtual: true` is the shorthand for `deploy: none` and keeps working. Writing
  both `virtual: true` and a contradicting `deploy` in one class is an error.
  `virtual: false` still claims nothing, since a boolean cannot tell "false" from
  "unset"; `deploy: container` is how to say it out loud and override another
  class. Only the field name is reserved, so a class may still be *called*
  `switch` and mean an ordinary container, as eight of the bundled examples do.
- **`boundary_class` on a group class**: names a connection class attached to
  every connection that leaves a group of that class. Whether the two ends sit
  in the same group follows from the topology, so annotating each edge would
  state twice what is already written once, and the two can disagree. The label
  goes on at the module tier, so a class written on the edge outranks it. The
  feature knows nothing about hosts: setting it on an `as` class marks the eBGP
  sessions just as setting it on the worker class marks the links that leave a
  machine.
- **`worker` group class**: marks a placement unit, a machine that containers
  are deployed onto, as opposed to a group that exists to share parameters. A
  node belongs to several groups at once, so the two uses need telling apart.
  dot2net owns the word rather than letting each platform module pick its own,
  because fourteen of the bundled examples emit both `topo.yaml` and `spec.yaml`
  from one topology and a name like `clabHost` in the DOT file would tie it to
  containerlab.
- **One containerlab topology file per machine.** A lab is deployed to a single
  machine, so when a scenario declares worker groups the topology file becomes
  group-scoped and each machine gets one it can deploy on its own, holding its
  own nodes and the links with both ends inside it. Bind paths become relative
  to the machine's directory, since containerlab resolves them against the
  directory holding the topology file; this requires
  `global.output_group_class: worker` so that a machine's files sit beside its
  topology, and says so if they do not. Scenarios that declare no placement unit
  keep the single network-scoped file.

  A link between two machines appears in no topology file, because nothing
  inside containerlab can create it — the machines are wired outside the lab.
  Give each machine its own bridge and join them with a link; that link is what
  `boundary_class` marks. Address assignment still sees one segment spanning
  both machines, because searching for a segment passes through bridges, which
  carry no addresses. `example/vlan_multihost` is built this way.
- **`LabelOwner.AddModuleClassLabels`**: lets a module attach a class label at the
  module tier. A module classifying objects through the `ObjectClassifier` hook
  had only `AddClassLabels`, which files labels as user-written — so a module's
  class would collide with the user's instead of losing to it, contrary to the
  rule that module-provided classes are the weakest.
- **`assert` module**: an opt-in module that checks the expectations a scenario
  states about itself. A class marked `values: {assert_used: "true"}` must be
  applied to at least one object, or the build fails. It generates no output.
  This closes a hole the golden tests cannot cover: a class that is declared but
  never applied contributes nothing, so regenerating the expected files silently
  freezes its absence — which is how `example/vlan_multihost` shipped with
  segment classes that never attached.
- **Group-scope file output**: a `groupclass` can now own a `file:` template, and
  `file` definitions accept `scope: group`. The file is written into the group's
  own directory (`cluster_h1/host.yaml`), mirroring how node-scope files land in
  the node's directory; `output: root` puts it at the output root instead, where
  `name_prefix` / `name_suffix` are needed to keep the groups apart.
- **Group child aggregation**: group-scope templates can reference
  `{{ .nodes_<name> }}` and `{{ .connections_<name> }}` and receive only the
  group's own members. A connection is included only when both of its endpoints
  are inside the group, so a link leaving the group appears in no group file.
- **`global.output_group_class`**: names the group class that splits the output
  directory. Every node of such a group has its files written below that group's
  directory, so a host packs as a single directory:
  `clabhost1/{topo.yaml, r1/frr.conf, r2/frr.conf}`. Network-scope files belong
  to no group and stay at the output root. A node in two groups of that class is
  reported as an error rather than silently assigned to one. Unset (the default)
  keeps the previous flat layout.

- **`values:` on segment classes**: `segmentclass` accepts a `values` map like the
  other class types, so a segment can carry static attributes
  (`values: {kind: ovs-bridge}`). Conflicting values from two classes of the same
  tier are rejected, as they are for the other class types.

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
  parameter**, but no example or module referenced it.
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

[Unreleased]: https://github.com/cpflat/dot2net/compare/v0.7.0...HEAD
[0.7.0]: https://github.com/cpflat/dot2net/compare/v0.6.2...v0.7.0
[0.6.2]: https://github.com/cpflat/dot2net/compare/v0.6.1...v0.6.2
[0.6.1]: https://github.com/cpflat/dot2net/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/cpflat/dot2net/compare/v0.5.1...v0.6.0
[0.5.1]: https://github.com/cpflat/dot2net/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/cpflat/dot2net/releases/tag/v0.5.0
