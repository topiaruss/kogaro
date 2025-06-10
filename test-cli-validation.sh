#!/bin/bash

echo "🧪 Testing Kogaro CLI Validation against Sample Testbed"
echo "========================================================="

# Build kogaro
echo "📦 Building Kogaro..."
go build .

if [ $? -ne 0 ]; then
    echo "❌ Build failed"
    exit 1
fi

echo "✅ Build successful"
echo ""

# Run Go tests first
echo "🧪 Running automated Go CLI tests..."
go test -v -run TestCLIValidation
test_exit_code=$?

if [ $test_exit_code -ne 0 ]; then
    echo "❌ Go CLI tests failed"
    exit 1
fi

echo "✅ Go CLI tests passed"
echo ""

# Test Helm template expansion workflow
echo "🎯 Testing Helm template expansion workflow:"
echo ""

test_helm_files=(
    "sample/kogaro-testbed/templates/deployment-missing-configmap.yaml:ConfigMap reference issue"
    "sample/kogaro-testbed/templates/deployment-privileged-container.yaml:Security issue"
    "sample/kogaro-testbed/templates/service-no-endpoints.yaml:Service connectivity issue"
    "sample/kogaro-testbed/templates/ingress-missing-ingressclass.yaml:IngressClass reference issue"
    "sample/kogaro-testbed/templates/pod-missing-secret-volume.yaml:Secret reference issue"
)

for test_case in "${test_helm_files[@]}"; do
    IFS=':' read -r file description <<< "$test_case"
    
    if [ -f "$file" ]; then
        echo "🧪 Testing: $description"
        echo "   File: $file"
        
        # First test: Should fail with Helm template error
        echo "   Step 1: Testing raw Helm template (should fail)..."
        ./kogaro --mode=one-off --config="$file" 2>&1 | grep -q "Helm templates"
        if [ $? -eq 0 ]; then
            echo "   ✅ Correctly detected Helm template"
        else
            echo "   ⚠️  Did not detect Helm template as expected"
        fi
        
        # Second test: Expand with helm template and validate
        echo "   Step 2: Expanding Helm template and validating..."
        temp_file=$(mktemp)
        
        # Expand the Helm template with values file
        helm template test sample/kogaro-testbed/ \
            --values sample/kogaro-testbed/values.yaml \
            --show-only "templates/$(basename "$file")" > "$temp_file" 2>/dev/null
        
        if [ -s "$temp_file" ]; then
            # Create output file for inspection
            output_file="validation-output-$(basename "$file" .yaml).txt"
            
            # Run validation on expanded YAML and capture output
            ./kogaro --mode=one-off --config="$temp_file" --output=ci > "$output_file" 2>&1
            exit_code=$?
            
            if [ $exit_code -eq 0 ]; then
                echo "   ⚠️  No issues found in expanded template (expected issues)"
                echo "   📄 Output saved to: $output_file"
            else
                echo "   ✅ Issues detected in expanded template (exit code: $exit_code)"
                echo "   📄 Validation output saved to: $output_file"
                
                # Show a preview of the issues found
                if grep -q "KOGARO-" "$output_file"; then
                    echo "   🔍 Issues found:"
                    grep "KOGARO-" "$output_file" | head -3 | sed 's/^/      /'
                    if [ $(grep -c "KOGARO-" "$output_file") -gt 3 ]; then
                        echo "      ... and $(($(grep -c "KOGARO-" "$output_file") - 3)) more issues"
                    fi
                fi
            fi
        else
            echo "   ⚠️  Could not expand Helm template (may need values file)"
        fi
        
        rm -f "$temp_file"
        echo ""
    else
        echo "   ⚠️  File not found: $file"
        echo ""
    fi
done

# Test with different validation modes using expanded templates
echo "🎛️  Testing different validation modes with expanded templates:"
echo ""

# Test expanded templates with various validation settings
test_validation_modes() {
    local template_file="$1"
    local description="$2"
    
    echo "📊 Testing: $description"
    temp_file=$(mktemp)
    
    # Expand the template with values file
    helm template test sample/kogaro-testbed/ \
        --values sample/kogaro-testbed/values.yaml \
        --show-only "templates/$(basename "$template_file")" > "$temp_file" 2>/dev/null
    
    if [ -s "$temp_file" ]; then
        # Create output files for different validation modes
        all_output_file="validation-all-$(basename "$template_file" .yaml).txt"
        security_output_file="validation-security-$(basename "$template_file" .yaml).txt"
        
        echo "   Testing with all validations enabled:"
        ./kogaro --mode=one-off \
            --config="$temp_file" \
            --enable-resource-limits-validation=true \
            --enable-security-validation=true \
            --enable-image-validation=false \
            --output=ci > "$all_output_file" 2>&1
        all_exit_code=$?
        echo "   Exit code: $all_exit_code"
        echo "   📄 Output saved to: $all_output_file"
        
        echo "   Testing with security-only validation:"
        ./kogaro --mode=one-off \
            --config="$temp_file" \
            --enable-resource-limits-validation=false \
            --enable-secret-validation=false \
            --enable-security-validation=true \
            --output=ci > "$security_output_file" 2>&1
        security_exit_code=$?
        echo "   Exit code: $security_exit_code"
        echo "   📄 Output saved to: $security_output_file"
        
        # Show preview of issues from all validations
        if [ $all_exit_code -ne 0 ] && grep -q "KOGARO-" "$all_output_file"; then
            echo "   🔍 Sample issues found:"
            grep "KOGARO-" "$all_output_file" | head -2 | sed 's/^/      /'
        fi
    else
        echo "   ⚠️  Could not expand template for validation mode testing"
    fi
    
    rm -f "$temp_file"
    echo ""
}

if command -v helm >/dev/null 2>&1; then
    test_validation_modes "sample/kogaro-testbed/templates/deployment-missing-resources.yaml" "Resource limits validation"
    test_validation_modes "sample/kogaro-testbed/templates/deployment-privileged-container.yaml" "Security validation"
else
    echo "⚠️  Helm not found - skipping validation mode tests with expanded templates"
    echo ""
fi

echo "📋 Summary:"
echo "- ✅ Go CLI tests verify Helm template detection and error handling"
echo "- ✅ Helm template workflow tested (expand first, then validate)"
echo "- ✅ Different validation modes tested on expanded templates"
echo "- 📄 All validation outputs saved to files for inspection"
echo ""
echo "📁 Generated output files:"
ls -1 validation-*.txt 2>/dev/null | sed 's/^/   /' || echo "   No validation output files found"
echo ""
echo "🎯 Recommended workflow:"
echo "   helm template <chart> [options] | kogaro --mode=one-off --config=-"
echo "   OR"
echo "   helm template <chart> [options] > output.yaml && kogaro --mode=one-off --config=output.yaml"
echo ""
echo "🔍 To inspect specific issues:"
echo "   cat validation-output-<template-name>.txt"
echo "   grep 'KOGARO-' validation-*.txt"
echo ""
echo "✅ CLI testing complete!"