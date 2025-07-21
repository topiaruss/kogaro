#!/bin/bash

# Helm Post-Renderer: Provenance Injector
# Adds source file provenance annotations to Helm-rendered resources
# Usage: helm template . --post-renderer ./helm-provenance-injector.sh

set -euo pipefail

current_source=""
in_metadata=false
annotations_found=false
provenance_added=false
line_number=0

while IFS= read -r line; do
    ((line_number++))
    
    # Detect source file comments
    if [[ $line =~ ^#\ Source:\ (.+)$ ]]; then
        current_source="${BASH_REMATCH[1]}"
        echo "$line"
        continue
    fi
    
    # Reset state on new resource
    if [[ $line =~ ^---$ ]]; then
        in_metadata=false
        annotations_found=false
        provenance_added=false
        echo "$line"
        continue
    fi
    
    # Reset state on new API version (start of resource)
    if [[ $line =~ ^apiVersion: ]]; then
        in_metadata=false
        annotations_found=false
        provenance_added=false
        echo "$line"
        continue
    fi
    
    # Track when we're in metadata section
    if [[ $line =~ ^metadata:$ ]]; then
        in_metadata=true
        echo "$line"
        continue
    fi
    
    # If we find existing annotations, add our provenance info right after
    if [[ $in_metadata == true ]] && [[ $line =~ ^[[:space:]]+annotations:[[:space:]]*$ ]] && [[ $provenance_added == false ]]; then
        annotations_found=true
        echo "$line"
        # Inject provenance annotations if we have source info
        if [[ -n $current_source ]]; then
            echo "    helm.provenance/source-file: \"$current_source\""
            echo "    helm.provenance/rendered-at: \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\""
            echo "    helm.provenance/line-number: \"$line_number\""
            provenance_added=true
        fi
        continue
    fi
    
    # If we're leaving metadata section and no annotations existed, create them
    if [[ $in_metadata == true ]] && [[ $line =~ ^spec: ]] && [[ $annotations_found == false ]] && [[ $provenance_added == false ]]; then
        # Add annotations before spec
        if [[ -n $current_source ]]; then
            echo "  annotations:"
            echo "    helm.provenance/source-file: \"$current_source\""
            echo "    helm.provenance/rendered-at: \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\""
            echo "    helm.provenance/line-number: \"$line_number\""
            provenance_added=true
        fi
        in_metadata=false
        echo "$line"
        continue
    fi
    
    # Check if we're exiting metadata (any non-indented line after metadata)
    if [[ $in_metadata == true ]] && [[ $line =~ ^[a-zA-Z] ]] && [[ $annotations_found == false ]] && [[ $provenance_added == false ]]; then
        # Add annotations before this line
        if [[ -n $current_source ]]; then
            echo "  annotations:"
            echo "    helm.provenance/source-file: \"$current_source\""
            echo "    helm.provenance/rendered-at: \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\""
            echo "    helm.provenance/line-number: \"$line_number\""
            provenance_added=true
        fi
        in_metadata=false
    fi
    
    echo "$line"
done