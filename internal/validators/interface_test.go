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
				Severity:       SeverityError,
				Details:        map[string]string{"container": "app"},
			},
			want: ValidationError{
				ResourceType:   "Pod",
				ResourceName:   "test-pod",
				Namespace:      "default",
				ValidationType: "missing_resource_requests",
				Message:        "Container 'app' has no resource requests defined",
				Severity:       SeverityError,
				Details:        map[string]string{"container": "app"},
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
				Severity:       SeverityError,
			},
			want: ValidationError{
				ResourceType:   "IngressClass",
				ResourceName:   "nginx",
				Namespace:      "",
				ValidationType: "dangling_ingress_class",
				Message:        "IngressClass 'nginx' does not exist",
				Severity:       SeverityError,
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

// Test the new ValidationError constructor and helper methods
func TestNewValidationError(t *testing.T) {
	err := NewValidationError("Pod", "test-pod", "default", "missing_resources", "Test message")
	
	if err.ResourceType != "Pod" {
		t.Errorf("ResourceType = %v, want %v", err.ResourceType, "Pod")
	}
	if err.ResourceName != "test-pod" {
		t.Errorf("ResourceName = %v, want %v", err.ResourceName, "test-pod")
	}
	if err.Namespace != "default" {
		t.Errorf("Namespace = %v, want %v", err.Namespace, "default")
	}
	if err.ValidationType != "missing_resources" {
		t.Errorf("ValidationType = %v, want %v", err.ValidationType, "missing_resources")
	}
	if err.Message != "Test message" {
		t.Errorf("Message = %v, want %v", err.Message, "Test message")
	}
	if err.Severity != SeverityError {
		t.Errorf("Severity = %v, want %v", err.Severity, SeverityError)
	}
	if err.Details == nil {
		t.Error("Details should be initialized")
	}
}

func TestValidationError_MethodChaining(t *testing.T) {
	err := NewValidationError("Service", "api-service", "production", "no_endpoints", "Service has no endpoints").
		WithSeverity(SeverityWarning).
		WithRemediationHint("Check that pods matching the service selector are running and ready").
		WithRelatedResources("Pod/api-pod-1", "Pod/api-pod-2").
		WithDetail("selector", "app=api").
		WithDetail("port", "8080")
	
	if err.Severity != SeverityWarning {
		t.Errorf("Severity = %v, want %v", err.Severity, SeverityWarning)
	}
	if err.RemediationHint != "Check that pods matching the service selector are running and ready" {
		t.Errorf("RemediationHint = %v, want expected hint", err.RemediationHint)
	}
	if len(err.RelatedResources) != 2 {
		t.Errorf("RelatedResources length = %v, want %v", len(err.RelatedResources), 2)
	}
	if err.RelatedResources[0] != "Pod/api-pod-1" {
		t.Errorf("RelatedResources[0] = %v, want %v", err.RelatedResources[0], "Pod/api-pod-1")
	}
	if err.Details["selector"] != "app=api" {
		t.Errorf("Details[selector] = %v, want %v", err.Details["selector"], "app=api")
	}
	if err.Details["port"] != "8080" {
		t.Errorf("Details[port] = %v, want %v", err.Details["port"], "8080")
	}
}

func TestValidationError_SeverityMethods(t *testing.T) {
	tests := []struct {
		name        string
		severity    Severity
		wantIsError bool
		wantIsWarn  bool
		wantIsInfo  bool
	}{
		{
			name:        "error severity",
			severity:    SeverityError,
			wantIsError: true,
			wantIsWarn:  false,
			wantIsInfo:  false,
		},
		{
			name:        "warning severity",
			severity:    SeverityWarning,
			wantIsError: false,
			wantIsWarn:  true,
			wantIsInfo:  false,
		},
		{
			name:        "info severity",
			severity:    SeverityInfo,
			wantIsError: false,
			wantIsWarn:  false,
			wantIsInfo:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationError("Pod", "test", "default", "test_type", "test message").
				WithSeverity(tt.severity)
			
			if err.IsError() != tt.wantIsError {
				t.Errorf("IsError() = %v, want %v", err.IsError(), tt.wantIsError)
			}
			if err.IsWarning() != tt.wantIsWarn {
				t.Errorf("IsWarning() = %v, want %v", err.IsWarning(), tt.wantIsWarn)
			}
			if err.IsInfo() != tt.wantIsInfo {
				t.Errorf("IsInfo() = %v, want %v", err.IsInfo(), tt.wantIsInfo)
			}
		})
	}
}

func TestValidationError_GetResourceKey(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		resource  string
		want      string
	}{
		{
			name:      "namespaced resource",
			namespace: "production",
			resource:  "api-service",
			want:      "production/api-service",
		},
		{
			name:      "cluster-scoped resource",
			namespace: "",
			resource:  "nginx-ingress-class",
			want:      "nginx-ingress-class",
		},
		{
			name:      "default namespace",
			namespace: "default",
			resource:  "test-pod",
			want:      "default/test-pod",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationError("TestResource", tt.resource, tt.namespace, "test_type", "test message")
			if got := err.GetResourceKey(); got != tt.want {
				t.Errorf("GetResourceKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidationError_WithRelatedResources(t *testing.T) {
	// Test adding multiple resources at once
	err := NewValidationError("Service", "api", "default", "test", "test").
		WithRelatedResources("Pod/pod1", "Pod/pod2", "Deployment/api-deployment")
	
	if len(err.RelatedResources) != 3 {
		t.Errorf("RelatedResources length = %v, want %v", len(err.RelatedResources), 3)
	}
	
	// Test adding resources in multiple calls
	err2 := NewValidationError("Service", "api", "default", "test", "test").
		WithRelatedResources("Pod/pod1").
		WithRelatedResources("Pod/pod2")
	
	if len(err2.RelatedResources) != 2 {
		t.Errorf("RelatedResources length = %v, want %v", len(err2.RelatedResources), 2)
	}
}

func TestValidationError_WithDetail(t *testing.T) {
	// Test that Details map is properly initialized and updated
	err := NewValidationError("Pod", "test", "default", "test", "test").
		WithDetail("container", "nginx").
		WithDetail("image", "nginx:latest")
	
	if len(err.Details) != 2 {
		t.Errorf("Details length = %v, want %v", len(err.Details), 2)
	}
	
	if err.Details["container"] != "nginx" {
		t.Errorf("Details[container] = %v, want %v", err.Details["container"], "nginx")
	}
	
	if err.Details["image"] != "nginx:latest" {
		t.Errorf("Details[image] = %v, want %v", err.Details["image"], "nginx:latest")
	}
	
	// Test overwriting existing detail
	err = err.WithDetail("container", "apache")
	if err.Details["container"] != "apache" {
		t.Errorf("Details[container] after overwrite = %v, want %v", err.Details["container"], "apache")
	}
}

func TestSeverityConstants(t *testing.T) {
	if SeverityError != "error" {
		t.Errorf("SeverityError = %v, want %v", SeverityError, "error")
	}
	if SeverityWarning != "warning" {
		t.Errorf("SeverityWarning = %v, want %v", SeverityWarning, "warning")
	}
	if SeverityInfo != "info" {
		t.Errorf("SeverityInfo = %v, want %v", SeverityInfo, "info")
	}
}