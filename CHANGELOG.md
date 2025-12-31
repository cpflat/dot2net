# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/cpflat/dot2net/compare/v0.6.2...HEAD
[0.6.2]: https://github.com/cpflat/dot2net/compare/v0.6.1...v0.6.2
[0.6.1]: https://github.com/cpflat/dot2net/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/cpflat/dot2net/compare/v0.5.1...v0.6.0
[0.5.1]: https://github.com/cpflat/dot2net/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/cpflat/dot2net/releases/tag/v0.5.0
