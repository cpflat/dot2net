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

# Check if specific scenario is provided as argument
if [ $# -eq 1 ]; then
    # Single scenario specified
    target_scenario="$1"
    scenario_dir="$EXAMPLE_DIR/$target_scenario"
    
    if [ ! -d "$scenario_dir" ]; then
        echo "Error: Scenario directory '$scenario_dir' not found"
        exit 1
    fi
    
    dot_file="$scenario_dir/input.dot"
    yaml_file="$scenario_dir/input.yaml"
    
    if [ ! -f "$dot_file" ] || [ ! -f "$yaml_file" ]; then
        echo "Error: Scenario '$target_scenario' missing input.dot or input.yaml"
        exit 1
    fi
    
    scenarios=("$target_scenario")
    echo "Processing single scenario: $target_scenario"
elif [ $# -eq 0 ]; then
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
else
    echo "Usage: $0 [scenario_name]"
    echo "  If no scenario_name is provided, all scenarios will be processed"
    exit 1
fi
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
    
    echo "  Getting list of files that will be generated..."
    # Get list of files that would be generated
    generated_files=$("$DOT2NET_BIN" files -c input.yaml input.dot)
    
    echo "  Running dot2net build..."
    "$DOT2NET_BIN" build -c input.yaml input.dot
    
    # Remove all files except the generated ones
    # First, remove input files
    rm -f input.dot input.yaml
    
    # Remove template files that were copied from scenario directory
    for file in "$scenario_dir"/*; do
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