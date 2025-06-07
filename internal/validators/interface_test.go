// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

package validators

import (
	"reflect"
	"testing"
)

func TestValidationError_Structure(t *testing.T) {
	tests := []struct {
		name string
		err  ValidationError
		want ValidationError
	}{
		{
			name: "complete validation error",
			err: ValidationError{
				ResourceType:   "Pod",
				ResourceName:   "test-pod",
				Namespace:      "default",
				ValidationType: "missing_resource_requests",
				Message:        "Container 'app' has no resource requests defined",
			},
			want: ValidationError{
				ResourceType:   "Pod",
				ResourceName:   "test-pod",
				Namespace:      "default",
				ValidationType: "missing_resource_requests",
				Message:        "Container 'app' has no resource requests defined",
			},
		},
		{
			name: "cluster-scoped resource error",
			err: ValidationError{
				ResourceType:   "IngressClass",
				ResourceName:   "nginx",
				Namespace:      "",
				ValidationType: "dangling_ingress_class",
				Message:        "IngressClass 'nginx' does not exist",
			},
			want: ValidationError{
				ResourceType:   "IngressClass",
				ResourceName:   "nginx",
				Namespace:      "",
				ValidationType: "dangling_ingress_class",
				Message:        "IngressClass 'nginx' does not exist",
			},
		},
		{
			name: "empty validation error",
			err:  ValidationError{},
			want: ValidationError{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.err, tt.want) {
				t.Errorf("ValidationError = %v, want %v", tt.err, tt.want)
			}

			// Test that all expected fields are accessible
			_ = tt.err.ResourceType
			_ = tt.err.ResourceName
			_ = tt.err.Namespace
			_ = tt.err.ValidationType
			_ = tt.err.Message
		})
	}
}

func TestValidationError_Fields(t *testing.T) {
	err := ValidationError{
		ResourceType:   "Deployment",
		ResourceName:   "app-deployment",
		Namespace:      "production",
		ValidationType: "missing_resource_limits",
		Message:        "Container 'web' has no resource limits defined",
	}

	// Test individual field access
	if err.ResourceType != "Deployment" {
		t.Errorf("ResourceType = %v, want %v", err.ResourceType, "Deployment")
	}
	if err.ResourceName != "app-deployment" {
		t.Errorf("ResourceName = %v, want %v", err.ResourceName, "app-deployment")
	}
	if err.Namespace != "production" {
		t.Errorf("Namespace = %v, want %v", err.Namespace, "production")
	}
	if err.ValidationType != "missing_resource_limits" {
		t.Errorf("ValidationType = %v, want %v", err.ValidationType, "missing_resource_limits")
	}
	if err.Message != "Container 'web' has no resource limits defined" {
		t.Errorf("Message = %v, want %v", err.Message, "Container 'web' has no resource limits defined")
	}
}

func TestValidationError_TypeConsistency(t *testing.T) {
	// Test that ValidationError fields are the expected types
	var err ValidationError
	
	if reflect.TypeOf(err.ResourceType).Kind() != reflect.String {
		t.Errorf("ResourceType should be string, got %v", reflect.TypeOf(err.ResourceType).Kind())
	}
	if reflect.TypeOf(err.ResourceName).Kind() != reflect.String {
		t.Errorf("ResourceName should be string, got %v", reflect.TypeOf(err.ResourceName).Kind())
	}
	if reflect.TypeOf(err.Namespace).Kind() != reflect.String {
		t.Errorf("Namespace should be string, got %v", reflect.TypeOf(err.Namespace).Kind())
	}
	if reflect.TypeOf(err.ValidationType).Kind() != reflect.String {
		t.Errorf("ValidationType should be string, got %v", reflect.TypeOf(err.ValidationType).Kind())
	}
	if reflect.TypeOf(err.Message).Kind() != reflect.String {
		t.Errorf("Message should be string, got %v", reflect.TypeOf(err.Message).Kind())
	}
}

// Test that ValidationError can be used as expected in validation scenarios
func TestValidationError_UsagePatterns(t *testing.T) {
	tests := []struct {
		name           string
		resourceType   string
		resourceName   string
		namespace      string
		validationType string
		message        string
		wantEmpty      bool
	}{
		{
			name:           "typical pod validation error",
			resourceType:   "Pod",
			resourceName:   "web-pod-123",
			namespace:      "default",
			validationType: "missing_security_context",
			message:        "Pod has no SecurityContext defined",
			wantEmpty:      false,
		},
		{
			name:           "service validation error with complex message",
			resourceType:   "Service",
			resourceName:   "api-service",
			namespace:      "api",
			validationType: "service_selector_mismatch",
			message:        "Service selector {app: api, version: v1} does not match any pods",
			wantEmpty:      false,
		},
		{
			name:           "cluster-scoped resource",
			resourceType:   "ClusterRole",
			resourceName:   "admin-role",
			namespace:      "",
			validationType: "excessive_permissions",
			message:        "ClusterRole grants excessive permissions",
			wantEmpty:      false,
		},
		{
			name:           "empty error",
			resourceType:   "",
			resourceName:   "",
			namespace:      "",
			validationType: "",
			message:        "",
			wantEmpty:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidationError{
				ResourceType:   tt.resourceType,
				ResourceName:   tt.resourceName,
				Namespace:      tt.namespace,
				ValidationType: tt.validationType,
				Message:        tt.message,
			}

			isEmpty := err.ResourceType == "" && err.ResourceName == "" && 
				err.Namespace == "" && err.ValidationType == "" && err.Message == ""

			if isEmpty != tt.wantEmpty {
				t.Errorf("ValidationError isEmpty = %v, want %v", isEmpty, tt.wantEmpty)
			}

			// Test that error can be used in collections
			errors := []ValidationError{err}
			if len(errors) != 1 {
				t.Errorf("ValidationError should work in slices")
			}

			// Test that error can be used in maps
			errorMap := map[string]ValidationError{"test": err}
			if len(errorMap) != 1 {
				t.Errorf("ValidationError should work as map values")
			}
		})
	}
}