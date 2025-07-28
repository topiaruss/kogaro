# Kogaro Configuration Management via Custom Resource Definition (CRD)

## Overview

This document outlines the proposed approach for making Kogaro configuration more dynamic and user-friendly through a Custom Resource Definition (CRD). This will allow real-time configuration updates without requiring pod restarts and provide better integration with the Kubernetes ecosystem.

## Problem Statement

Currently, Kogaro configuration is hardcoded in the application, requiring:
- Code changes and rebuilds for configuration updates
- Pod restarts for changes to take effect
- No easy way to manage different configurations
- Limited integration with Kubernetes tooling

## Proposed Solution: CRD-Based Configuration

### Benefits

1. **Kubernetes Native**: Fits perfectly with the Kubernetes ecosystem
2. **Real-time Updates**: Can watch for changes and reload configuration without restarts
3. **Tool Integration**: Works with kubectl, Lens, ArgoCD, etc.
4. **Version Controlled**: Configuration stored in cluster with full audit trail
5. **Multiple Configurations**: Support for different configs per namespace/environment
6. **Future-proof**: Can evolve to support more complex configurations

### CRD Structure

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: kogaroconfigs.kogaro.k8s.io
spec:
  group: kogaro.k8s.io
  names:
    kind: KogaroConfig
    listKind: KogaroConfigList
    plural: kogaroconfigs
    singular: kogaroconfig
  scope: Namespaced
  versions:
    - name: v1alpha1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                namespaceExclusions:
                  type: object
                  properties:
                    systemNamespaces:
                      type: array
                      items:
                        type: string
                    securityExcludedNamespaces:
                      type: array
                      items:
                        type: string
                    networkingExcludedNamespaces:
                      type: array
                      items:
                        type: string
                validationSettings:
                  type: object
                  properties:
                    enableSecurityValidation:
                      type: boolean
                    enableNetworkingValidation:
                      type: boolean
                    enableResourceValidation:
                      type: boolean
                    enableImageValidation:
                      type: boolean
                    enableReferenceValidation:
                      type: boolean
                thresholds:
                  type: object
                  properties:
                    minCpuRequest:
                      type: string
                    minMemoryRequest:
                      type: string
                    minCpuLimit:
                      type: string
                    minMemoryLimit:
                      type: string
                securitySettings:
                  type: object
                  properties:
                    enableRootUserValidation:
                      type: boolean
                    enableSecurityContextValidation:
                      type: boolean
                    enableServiceAccountValidation:
                      type: boolean
                    enableNetworkPolicyValidation:
                      type: boolean
                    securitySensitiveNamespaces:
                      type: array
                      items:
                        type: string
                networkingSettings:
                  type: object
                  properties:
                    enableServiceValidation:
                      type: boolean
                    enableNetworkPolicyValidation:
                      type: boolean
                    enableIngressValidation:
                      type: boolean
                    warnUnexposedPods:
                      type: boolean
                    policyRequiredNamespaces:
                      type: array
                      items:
                        type: string
```

### Example Configuration

```yaml
apiVersion: kogaro.k8s.io/v1alpha1
kind: KogaroConfig
metadata:
  name: default
  namespace: kogaro-system
spec:
  namespaceExclusions:
    systemNamespaces:
      - kube-system
      - kube-public
      - kube-node-lease
      - default
      - monitoring
      - cert-manager
      - kogaro-system
    securityExcludedNamespaces:
      - kube-system
      - kube-public
      - kube-node-lease
      - monitoring
      - cert-manager
      - kogaro-system
    networkingExcludedNamespaces:
      - kube-system
      - kube-public
      - kube-node-lease
      - monitoring
      - cert-manager
      - kogaro-system
  validationSettings:
    enableSecurityValidation: true
    enableNetworkingValidation: true
    enableResourceValidation: true
    enableImageValidation: true
    enableReferenceValidation: true
  thresholds:
    minCpuRequest: "100m"
    minMemoryRequest: "128Mi"
    minCpuLimit: "200m"
    minMemoryLimit: "256Mi"
  securitySettings:
    enableRootUserValidation: true
    enableSecurityContextValidation: true
    enableServiceAccountValidation: true
    enableNetworkPolicyValidation: true
    securitySensitiveNamespaces:
      - droplinks-production
      - droplinks-staging
  networkingSettings:
    enableServiceValidation: true
    enableNetworkPolicyValidation: true
    enableIngressValidation: true
    warnUnexposedPods: false
    policyRequiredNamespaces:
      - droplinks-production
      - droplinks-staging
```

## Implementation Plan

### Phase 1: CRD Definition and Basic Controller
1. **Define CRD**: Create the CustomResourceDefinition
2. **Basic Controller**: Implement controller to watch for CRD changes
3. **Configuration Loading**: Modify validators to read from CRD
4. **Fallback Logic**: Maintain backward compatibility with hardcoded defaults

### Phase 2: Enhanced Features
1. **Validation**: Add CRD validation for configuration values
2. **Status Updates**: Add status field to track configuration application
3. **Multiple Configs**: Support for different configurations per namespace
4. **Migration Tool**: Tool to migrate from hardcoded to CRD-based config

### Phase 3: Tooling and Integration
1. **kubectl Plugin**: Create kubectl-kogaro plugin for easy management
2. **Lens Integration**: Develop Lens plugin for visual configuration
3. **ArgoCD Integration**: Support for GitOps workflows
4. **Documentation**: Comprehensive documentation and examples

## Alternative Approaches Considered

### Option 1: Helm Chart Configuration
**Pros:**
- Familiar to Kubernetes users
- Version controlled configuration
- Easy rollback
- No additional infrastructure needed

**Cons:**
- Requires pod restart for changes
- Not real-time
- Configuration drift possible

### Option 2: Client Tool (CLI)
**Pros:**
- Familiar CLI experience
- Can validate configuration
- Can show current state
- Easy to script/automate

**Cons:**
- Another tool to maintain
- Requires authentication setup
- Not integrated with existing tools

### Option 3: Lens Plugin
**Pros:**
- Great UX for Lens users
- Visual configuration
- Integrated with cluster view

**Cons:**
- Only works for Lens users
- Requires plugin development
- Limited to Lens ecosystem

## Migration Strategy

### Step 1: Backward Compatibility
- Maintain existing hardcoded configuration as defaults
- CRD configuration overrides hardcoded defaults
- Gradual migration path

### Step 2: Configuration Migration
- Provide migration tool to convert hardcoded config to CRD
- Document migration process
- Support rollback to hardcoded config if needed

### Step 3: Full CRD Adoption
- Remove hardcoded configuration
- Require CRD for all configuration
- Update documentation and examples

## Benefits for Users

1. **Real-time Configuration**: No more pod restarts for config changes
2. **Kubernetes Native**: Use familiar kubectl commands
3. **Version Control**: Configuration changes tracked in cluster
4. **Tool Integration**: Works with existing Kubernetes tooling
5. **Multiple Environments**: Different configs for different namespaces
6. **Audit Trail**: Full history of configuration changes

## Next Steps

1. **Implement CRD**: Start with basic CRD definition
2. **Controller Development**: Build controller to watch for changes
3. **Validator Integration**: Modify validators to use CRD config
4. **Testing**: Comprehensive testing with different configurations
5. **Documentation**: Update documentation with CRD examples
6. **Migration Guide**: Create guide for existing users

This approach provides the most flexibility and best integration with the Kubernetes ecosystem while maintaining simplicity for users. 