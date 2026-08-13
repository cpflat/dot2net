#!/bin/bash

# Script to generate expected output directories for all dot2net topologies
# This script is useful for updating expected test results when dot2net specifications change
#
# Topologies live under two roots: topologies/ holds the ones worth deploying,
# example/ the ones that demonstrate a notation. Both are golden-tested.

set -e

# Get the absolute path to the dot2net project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
TOPOLOGY_ROOTS=("$PROJECT_ROOT/topologies" "$PROJECT_ROOT/example")
DOT2NET_BIN="$PROJECT_ROOT/dot2net"

echo "Project root: $PROJECT_ROOT"
echo "Topology roots: ${TOPOLOGY_ROOTS[*]}"

# Check if dot2net binary exists
if [ ! -f "$DOT2NET_BIN" ]; then
    echo "Error: dot2net binary not found at $DOT2NET_BIN"
    echo "Please build dot2net first with 'go build -o dot2net'"
    exit 1
fi

# Check if a specific topology is provided as argument
if [ $# -eq 1 ]; then
    # Single topology specified; find which root it sits under
    target_topology="$1"
    topology_dirs=()
    for root in "${TOPOLOGY_ROOTS[@]}"; do
        candidate="$root/$target_topology"
        if [ -f "$candidate/input.dot" ] && [ -f "$candidate/input.yaml" ]; then
            topology_dirs=("$candidate")
            break
        fi
    done

    if [ ${#topology_dirs[@]} -eq 0 ]; then
        echo "Error: topology '$target_topology' not found under ${TOPOLOGY_ROOTS[*]}"
        exit 1
    fi

    echo "Processing single topology: $target_topology"
elif [ $# -eq 0 ]; then
    # Find all topologies with input.dot and input.yaml
    topology_dirs=()
    for root in "${TOPOLOGY_ROOTS[@]}"; do
        for dir in "$root"/*; do
            if [ -d "$dir" ] && [ -f "$dir/input.dot" ] && [ -f "$dir/input.yaml" ]; then
                topology_dirs+=("$dir")
            fi
        done
    done

    if [ ${#topology_dirs[@]} -eq 0 ]; then
        echo "Error: No valid topologies found with input.dot and input.yaml"
        exit 1
    fi

    echo "Found ${#topology_dirs[@]} topologies"
else
    echo "Usage: $0 [topology_name]"
    echo "  If no topology_name is provided, all topologies will be processed"
    exit 1
fi
echo

# Process each topology
for topology_dir in "${topology_dirs[@]}"; do
    topology=$(basename "$topology_dir")
    echo "Processing topology: $topology"
    expected_dir="$topology_dir/expected"
    
    # Create temporary directory for dot2net execution
    # An explicit template is required: BSD mktemp with no template ignores
    # TMPDIR and uses the Darwin per-user temp dir instead.
    temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/dot2net_expected.XXXXXX")
    trap "rm -rf $temp_dir" EXIT
    
    # Copy input files and any template files to temp directory
    cp "$topology_dir/input.dot" "$temp_dir/"
    cp "$topology_dir/input.yaml" "$temp_dir/"
    
    # Copy any additional template files (but exclude subdirectories and legacy files)
    for file in "$topology_dir"/*; do
        if [ -f "$file" ]; then
            filename=$(basename "$file")
            # Skip input files (already copied) and legacy/backup files
            if [[ "$filename" != "input.dot" && "$filename" != "input.yaml" && 
                  "$filename" != *.legacy && "$filename" != *.bak && 
                  "$filename" != *.pdf ]]; then
                cp "$file" "$temp_dir/"
            fi
        fi
    done
    
    # Change to temp directory and run dot2net
    cd "$temp_dir"
    
    echo "  Getting list of files that will be generated..."
    # Get list of files that would be generated
    generated_files=$("$DOT2NET_BIN" files -c input.yaml input.dot)
    
    echo "  Running dot2net build..."
    "$DOT2NET_BIN" build -c input.yaml input.dot
    
    # Remove all files except the generated ones
    # First, remove input files
    rm -f input.dot input.yaml
    
    # Remove template files that were copied from the topology directory
    for file in "$topology_dir"/*; do
        if [ -f "$file" ]; then
            filename=$(basename "$file")
            # Skip input files (already removed) and generated outputs
            case "$filename" in
                "input.dot"|"input.yaml"|*.legacy|*.bak|*.pdf)
                    rm -f "$temp_dir/$filename" 2>/dev/null || true
                    ;;
            esac
        fi
    done
    
    # Keep only the files that were supposed to be generated
    # Create a temporary list of files to keep
    echo "$generated_files" > expected_files.txt
    
    # Remove any files/directories not in the generated list
    for item in *; do
        if [ -f "$item" ] || [ -d "$item" ]; then
            # Check if this item (or any file within it) is in the expected list
            found=false
            while IFS= read -r expected_file; do
                if [ "$item" = "$expected_file" ] || [[ "$expected_file" == "$item/"* ]]; then
                    found=true
                    break
                fi
            done < expected_files.txt
            
            # If not found in expected files, remove it (except our helper file)
            if [ "$found" = false ] && [ "$item" != "expected_files.txt" ]; then
                rm -rf "$item" 2>/dev/null || true
            fi
        fi
    done
    
    # Clean up helper file
    rm -f expected_files.txt
    
    # Create/update expected directory
    if [ -d "$expected_dir" ]; then
        echo "  Updating existing expected directory..."
        rm -rf "$expected_dir"
    else
        echo "  Creating new expected directory..."
    fi
    
    mkdir -p "$expected_dir"
    
    # Copy all generated files to expected directory
    if [ "$(ls -A "$temp_dir")" ]; then
        cp -r "$temp_dir"/* "$expected_dir/"
        echo "  Generated expected files for $topology"
    else
        echo "  Warning: No output files generated for $topology"
    fi
    
    # Clean up temp directory for next iteration
    rm -rf "$temp_dir"
    echo
done

echo "Finished generating expected outputs for all topologies"
echo "You can now run the tests with: go test ./internal/test/..."