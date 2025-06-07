// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

package validators

// Common test helper functions used across multiple test files

// boolPtr returns a pointer to a bool value
func boolPtr(b bool) *bool {
	return &b
}

// stringPtr returns a pointer to a string value
func stringPtr(s string) *string {
	return &s
}

// int64Ptr returns a pointer to an int64 value
func int64Ptr(i int64) *int64 {
	return &i
}

