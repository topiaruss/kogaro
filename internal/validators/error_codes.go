// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

package validators

// ErrorCodeRegistry provides centralized error code mapping for all validators.
// This eliminates scattered switch statements and provides a single source of truth.
type ErrorCodeRegistry struct {
	// Simple validation type → error code mappings
	codes map[string]string
}

// NewErrorCodeRegistry creates and initializes the error code registry.
func NewErrorCodeRegistry() *ErrorCodeRegistry {
	registry := &ErrorCodeRegistry{
		codes: make(map[string]string),
	}
	registry.registerAllCodes()
	return registry
}

// registerAllCodes registers all error codes from all validators.
func (r *ErrorCodeRegistry) registerAllCodes() {
	// Networking Validator (NET)
	r.codes["networking:service_selector_mismatch"] = "KOGARO-NET-001"
	r.codes["networking:service_no_endpoints"] = "KOGARO-NET-002"
	r.codes["networking:service_port_mismatch"] = "KOGARO-NET-003"
	r.codes["networking:pod_no_service"] = "KOGARO-NET-004"
	r.codes["networking:network_policy_orphaned"] = "KOGARO-NET-005"
	r.codes["networking:missing_network_policy_default_deny"] = "KOGARO-NET-006"
	r.codes["networking:ingress_service_missing"] = "KOGARO-NET-007"
	r.codes["networking:ingress_service_port_mismatch"] = "KOGARO-NET-008"
	r.codes["networking:ingress_no_backend_pods"] = "KOGARO-NET-009"

	// Security Validator (SEC)
	r.codes["security:pod_running_as_root"] = "KOGARO-SEC-001"
	r.codes["security:pod_allows_root_user"] = "KOGARO-SEC-002"
	r.codes["security:container_running_as_root"] = "KOGARO-SEC-003"
	r.codes["security:container_allows_privilege_escalation"] = "KOGARO-SEC-004"
	r.codes["security:container_allows_privilege_escalation:privileged"] = "KOGARO-SEC-005"
	r.codes["security:container_privileged_mode"] = "KOGARO-SEC-006"
	r.codes["security:container_writable_root_filesystem"] = "KOGARO-SEC-007"
	r.codes["security:container_additional_capabilities"] = "KOGARO-SEC-008"
	r.codes["security:missing_pod_security_context"] = "KOGARO-SEC-009"
	r.codes["security:missing_container_security_context"] = "KOGARO-SEC-010"
	r.codes["security:serviceaccount_cluster_role_binding"] = "KOGARO-SEC-011"
	r.codes["security:serviceaccount_excessive_permissions"] = "KOGARO-SEC-012"

	// Resource Limits Validator (RES)
	r.codes["resource_limits:missing_resource_requests:Deployment"] = "KOGARO-RES-001"
	r.codes["resource_limits:missing_resource_requests:StatefulSet"] = "KOGARO-RES-002"
	r.codes["resource_limits:missing_resource_limits:Deployment:no_requests"] = "KOGARO-RES-003"
	r.codes["resource_limits:missing_resource_limits:Deployment:has_requests"] = "KOGARO-RES-004"
	r.codes["resource_limits:missing_resource_limits:StatefulSet"] = "KOGARO-RES-005"
	r.codes["resource_limits:insufficient_cpu_request"] = "KOGARO-RES-006"
	r.codes["resource_limits:insufficient_memory_request"] = "KOGARO-RES-007"
	r.codes["resource_limits:qos_class_issue:Deployment:BestEffort"] = "KOGARO-RES-008"
	r.codes["resource_limits:qos_class_issue:StatefulSet:BestEffort"] = "KOGARO-RES-009"
	r.codes["resource_limits:qos_class_issue:Deployment:Burstable"] = "KOGARO-RES-010"

	// Reference Validator (REF)
	r.codes["reference:dangling_ingress_class"] = "KOGARO-REF-001"
	r.codes["reference:dangling_service_reference"] = "KOGARO-REF-002"
	r.codes["reference:dangling_configmap_volume"] = "KOGARO-REF-003"
	r.codes["reference:dangling_configmap_envfrom"] = "KOGARO-REF-004"
	r.codes["reference:dangling_secret_volume"] = "KOGARO-REF-005"
	r.codes["reference:dangling_secret_envfrom"] = "KOGARO-REF-006"
	r.codes["reference:dangling_secret_env"] = "KOGARO-REF-007"
	r.codes["reference:dangling_tls_secret"] = "KOGARO-REF-008"
	r.codes["reference:dangling_storage_class"] = "KOGARO-REF-009"
	r.codes["reference:dangling_pvc_reference"] = "KOGARO-REF-010"
	r.codes["reference:dangling_service_account"] = "KOGARO-REF-011"

	// Image Validator (IMG)
	r.codes["image:invalid_image_reference"] = "KOGARO-IMG-001"
	r.codes["image:missing_image"] = "KOGARO-IMG-002"
	r.codes["image:missing_image_warning"] = "KOGARO-IMG-003"
	r.codes["image:architecture_mismatch"] = "KOGARO-IMG-004"
	r.codes["image:architecture_mismatch_warning"] = "KOGARO-IMG-005"
}

// GetNetworkingErrorCode returns the error code for networking validation types.
func (r *ErrorCodeRegistry) GetNetworkingErrorCode(validationType string) string {
	if code, exists := r.codes["networking:"+validationType]; exists {
		return code
	}
	return "KOGARO-NET-UNKNOWN"
}

// GetSecurityErrorCode returns the error code for security validation types.
// context map can contain "is_privileged" for conditional logic.
func (r *ErrorCodeRegistry) GetSecurityErrorCode(validationType string, context map[string]interface{}) string {
	// Handle special case for privilege escalation
	if validationType == "container_allows_privilege_escalation" {
		if isPrivileged, ok := context["is_privileged"].(bool); ok && isPrivileged {
			if code, exists := r.codes["security:"+validationType+":privileged"]; exists {
				return code
			}
		}
	}

	if code, exists := r.codes["security:"+validationType]; exists {
		return code
	}
	return "KOGARO-SEC-UNKNOWN"
}

// GetResourceLimitsErrorCode returns the error code for resource limits validation types.
func (r *ErrorCodeRegistry) GetResourceLimitsErrorCode(validationType, resourceType, issueDetail string, hasRequests bool) string {
	// Try specific key combinations
	keys := []string{
		"resource_limits:" + validationType + ":" + resourceType + ":" + issueDetail,
		"resource_limits:" + validationType + ":" + resourceType,
		"resource_limits:" + validationType,
	}

	// Handle special case for missing_resource_limits
	if validationType == "missing_resource_limits" && resourceType == "Deployment" {
		if hasRequests {
			if code, exists := r.codes["resource_limits:missing_resource_limits:Deployment:has_requests"]; exists {
				return code
			}
		} else {
			if code, exists := r.codes["resource_limits:missing_resource_limits:Deployment:no_requests"]; exists {
				return code
			}
		}
	}

	for _, key := range keys {
		if code, exists := r.codes[key]; exists {
			return code
		}
	}
	return "KOGARO-RES-UNKNOWN"
}

// GetReferenceErrorCode returns the error code for reference validation types.
func (r *ErrorCodeRegistry) GetReferenceErrorCode(validationType string) string {
	if code, exists := r.codes["reference:"+validationType]; exists {
		return code
	}
	return "KOGARO-REF-UNKNOWN"
}

// GetImageErrorCode returns the error code for image validation types.
func (r *ErrorCodeRegistry) GetImageErrorCode(validationType string) string {
	if code, exists := r.codes["image:"+validationType]; exists {
		return code
	}
	return "KOGARO-IMG-UNKNOWN"
}

// Global error code registry instance
var globalErrorCodeRegistry = NewErrorCodeRegistry()

// GetNetworkingErrorCode is a package-level convenience function.
func GetNetworkingErrorCode(validationType string) string {
	return globalErrorCodeRegistry.GetNetworkingErrorCode(validationType)
}

// GetSecurityErrorCode is a package-level convenience function.
func GetSecurityErrorCode(validationType string, context map[string]interface{}) string {
	return globalErrorCodeRegistry.GetSecurityErrorCode(validationType, context)
}

// GetResourceLimitsErrorCode is a package-level convenience function.
func GetResourceLimitsErrorCode(validationType, resourceType, issueDetail string, hasRequests bool) string {
	return globalErrorCodeRegistry.GetResourceLimitsErrorCode(validationType, resourceType, issueDetail, hasRequests)
}

// GetReferenceErrorCode is a package-level convenience function.
func GetReferenceErrorCode(validationType string) string {
	return globalErrorCodeRegistry.GetReferenceErrorCode(validationType)
}

// GetImageErrorCode is a package-level convenience function.
func GetImageErrorCode(validationType string) string {
	return globalErrorCodeRegistry.GetImageErrorCode(validationType)
}
