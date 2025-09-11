#!/bin/bash

# Script to generate expected output directories for all dot2net example scenarios
# This script is useful for updating expected test results when dot2net specifications change

set -e

# Get the absolute path to the dot2net project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
EXAMPLE_DIR="$PROJECT_ROOT/example"
DOT2NET_BIN="$PROJECT_ROOT/dot2net"

echo "Project root: $PROJECT_ROOT"
echo "Example directory: $EXAMPLE_DIR"

# Check if dot2net binary exists
if [ ! -f "$DOT2NET_BIN" ]; then
    echo "Error: dot2net binary not found at $DOT2NET_BIN"
    echo "Please build dot2net first with 'go build -o dot2net'"
    exit 1
fi

# Find all scenarios with input.dot and input.yaml
scenarios=()
for dir in "$EXAMPLE_DIR"/*; do
    if [ -d "$dir" ]; then
        scenario_name=$(basename "$dir")
        dot_file="$dir/input.dot"
        yaml_file="$dir/input.yaml"
        
        if [ -f "$dot_file" ] && [ -f "$yaml_file" ]; then
            scenarios+=("$scenario_name")
        fi
    fi
done

if [ ${#scenarios[@]} -eq 0 ]; then
    echo "Error: No valid scenarios found with input.dot and input.yaml"
    exit 1
fi

echo "Found ${#scenarios[@]} scenarios: ${scenarios[*]}"
echo

# Process each scenario
for scenario in "${scenarios[@]}"; do
    echo "Processing scenario: $scenario"
    scenario_dir="$EXAMPLE_DIR/$scenario"
    expected_dir="$scenario_dir/expected"
    
    # Create temporary directory for dot2net execution
    temp_dir=$(mktemp -d)
    trap "rm -rf $temp_dir" EXIT
    
    # Copy input files and any template files to temp directory
    cp "$scenario_dir/input.dot" "$temp_dir/"
    cp "$scenario_dir/input.yaml" "$temp_dir/"
    
    # Copy any additional template files (but exclude subdirectories and legacy files)
    for file in "$scenario_dir"/*; do
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
    echo "  Running dot2net build..."
    "$DOT2NET_BIN" build -c input.yaml input.dot
    
    # Remove the original input and template files from temp directory
    # Keep only the generated output files
    rm -f input.dot input.yaml
    # Remove other template files that were copied
    for file in "$scenario_dir"/*; do
        if [ -f "$file" ]; then
            filename=$(basename "$file")
            if [[ "$filename" != "input.dot" && "$filename" != "input.yaml" && 
                  "$filename" != *.legacy && "$filename" != *.bak && 
                  "$filename" != *.pdf ]]; then
                rm -f "$temp_dir/$filename" 2>/dev/null || true
            fi
        fi
    done
    
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
        echo "  Generated expected files for $scenario"
    else
        echo "  Warning: No output files generated for $scenario"
    fi
    
    # Clean up temp directory for next iteration
    rm -rf "$temp_dir"
    echo
done

echo "Finished generating expected outputs for all scenarios"
echo "You can now run the tests with: go test ./internal/test/..."