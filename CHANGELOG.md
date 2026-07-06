# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

Bug fixes from a systematic code review (correctness, determinism, and robustness):

- **Deterministic output**: sort links, generated file lists, group creation, and parameter-rule processing so builds are byte-stable across runs (previously map / edge iteration order could vary)
- **`param_format` expansion**: `param_format` values are now expanded as `text/template` (e.g. `VLAN{{ .value }}`) instead of being assigned as the literal template string
- **Multiple NodeClass matching**: `NodeClassCheck` / `NeighborNodeClassCheck` no longer drop the plural `nodes:` / `neighbor_nodes:` list (a `copy` into a zero-length slice was a no-op)
- **Address block calculation**: `getitem` now carries into the most significant byte and handles prefix lengths ≤ 8 (previously it could ignore the index or drop the top-byte carry); `getAvailablePrefix` no longer overflows its scan bound for large (e.g. IPv6) pools
- **Bind mount source path**: tinet / containerlab bind mounts use the actual generated filename (`GetFileName`) instead of the file-definition id, fixing paths when `name_prefix` / `name_suffix` is set
- **Errors instead of panics**: return errors (rather than panicking) on missing dot-file arguments, malformed DOT input, ambiguous edges during diagram merge, out-of-range member / neighbor references, and insufficient `file` parameter candidates
- **Error propagation**: surface previously-discarded errors from place-label checks and management-interface labels, and fix `%w` error wrapping so failures carry their cause
- **Diagram merge**: merging diagrams no longer drops node groups that exist only in the second diagram
- **Containerlab endpoints**: no longer emit a dangling `endpoints:` line when a node has no non-virtual connections
- **Visual output**: guard against nil node / interface references and wrap the underlying error when an interface lacks an IP address
- **`suggestAlternativeName`**: suggest a genuinely different name for the `node_` / `group_` reserved prefixes (previously returned the same name)

### Added
- Regression tests for `getitem` address arithmetic, multiple-NodeClass matching, and `param_format` expansion

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
