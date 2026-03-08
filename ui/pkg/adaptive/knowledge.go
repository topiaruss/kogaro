package adaptive

import "strings"

// knownImages maps base image names to their known traits.
var knownImages = map[string]*ImageTraits{
	"nginx": {
		DefaultUID:    101,
		DefaultGID:    101,
		WritablePaths: []string{"/var/cache/nginx", "/var/run", "/tmp"},
		SafeForReadOnlyFS: false,
		Notes:         "Needs writable /var/cache/nginx for proxy cache and /var/run for pid file",
	},
	"postgres": {
		DefaultUID:    999,
		DefaultGID:    999,
		WritablePaths: []string{"/var/lib/postgresql/data", "/var/run/postgresql", "/tmp"},
		SafeForReadOnlyFS: false,
		Notes:         "Data directory must be writable; uses /var/run/postgresql for socket",
	},
	"bitnami/postgresql": {
		DefaultUID:    1001,
		DefaultGID:    1001,
		WritablePaths: []string{"/bitnami/postgresql", "/tmp", "/opt/bitnami"},
		SafeForReadOnlyFS: false,
		EnvPrefix:     "POSTGRESQL_",
		Notes:         "Bitnami images use UID 1001 by convention",
	},
	"redis": {
		DefaultUID:    999,
		DefaultGID:    999,
		WritablePaths: []string{"/data", "/tmp"},
		SafeForReadOnlyFS: false,
		Notes:         "Needs writable /data for persistence",
	},
	"alpine": {
		DefaultUID:        65534,
		DefaultGID:        65534,
		WritablePaths:     []string{"/tmp"},
		SafeForReadOnlyFS: true,
		Notes:             "Safe for readOnlyRootFilesystem when used as a static application base",
	},
	"gcr.io/distroless": {
		DefaultUID:        65534,
		DefaultGID:        65534,
		WritablePaths:     []string{"/tmp"},
		IsDistroless:      true,
		SafeForReadOnlyFS: true,
		Notes:             "Distroless images have no shell; ideal for read-only root filesystem",
	},
	"node": {
		DefaultUID:    1000,
		DefaultGID:    1000,
		WritablePaths: []string{"/tmp"},
		SafeForReadOnlyFS: false,
		Notes:         "node_modules may need to be writable depending on app; consider multi-stage builds",
	},
	"python": {
		DefaultUID:    65534,
		DefaultGID:    65534,
		WritablePaths: []string{"/tmp"},
		SafeForReadOnlyFS: false,
		Notes:         "pip install and __pycache__ may require writable directories",
	},
	"httpd": {
		DefaultUID:    33,
		DefaultGID:    33,
		WritablePaths: []string{"/usr/local/apache2/logs", "/tmp"},
		SafeForReadOnlyFS: false,
		Notes:         "Apache needs writable log directory",
	},
	"apache": {
		DefaultUID:    33,
		DefaultGID:    33,
		WritablePaths: []string{"/usr/local/apache2/logs", "/tmp"},
		SafeForReadOnlyFS: false,
		Notes:         "Alias for httpd; Apache needs writable log directory",
	},
	"mysql": {
		DefaultUID:    999,
		DefaultGID:    999,
		WritablePaths: []string{"/var/lib/mysql", "/tmp"},
		SafeForReadOnlyFS: false,
		Notes:         "Data directory must be writable",
	},
	"mongo": {
		DefaultUID:    999,
		DefaultGID:    999,
		WritablePaths: []string{"/data/db", "/tmp"},
		SafeForReadOnlyFS: false,
		Notes:         "Data directory must be writable for database storage",
	},
	"memcached": {
		DefaultUID:    11211,
		DefaultGID:    11211,
		WritablePaths: []string{"/tmp"},
		SafeForReadOnlyFS: true,
		Notes:         "In-memory cache; no persistent storage needed",
	},
}

// LookupImage returns known traits for a container image. It tries exact match,
// then base name match, then prefix match against the knowledge base.
func LookupImage(image string) (*ImageTraits, bool) {
	base, _ := ParseImageRef(image)

	// Exact match on base
	if traits, ok := knownImages[base]; ok {
		return traits, true
	}

	// Strip common registry prefixes and try again
	stripped := stripRegistry(base)
	if stripped != base {
		if traits, ok := knownImages[stripped]; ok {
			return traits, true
		}
	}

	// Strip "library/" prefix (docker.io/library/nginx -> nginx)
	stripped = strings.TrimPrefix(stripped, "library/")
	if traits, ok := knownImages[stripped]; ok {
		return traits, true
	}

	// Prefix match: check if any known key is a prefix of the base
	for key, traits := range knownImages {
		if strings.HasPrefix(base, key+"/") || strings.HasPrefix(stripped, key+"/") {
			return traits, true
		}
	}

	// Reverse prefix match: check if the base starts with a known key
	for key, traits := range knownImages {
		if strings.HasPrefix(base, key) && len(base) > len(key) && base[len(key)] == '/' {
			return traits, true
		}
		if strings.HasPrefix(stripped, key) && len(stripped) > len(key) && stripped[len(key)] == '/' {
			return traits, true
		}
	}

	return nil, false
}

// ParseImageRef splits an image reference into base and tag components.
// Examples:
//
//	"nginx:1.25"           -> ("nginx", "1.25")
//	"nginx"                -> ("nginx", "latest")
//	"reg.io/org/name:v1"   -> ("reg.io/org/name", "v1")
//	"reg.io/org/name@sha256:abc" -> ("reg.io/org/name", "sha256:abc")
func ParseImageRef(image string) (base, tag string) {
	// Handle digest references
	if idx := strings.LastIndex(image, "@"); idx != -1 {
		return image[:idx], image[idx+1:]
	}

	// Find the last colon, but be careful with registry ports (e.g., reg.io:5000/name:tag)
	// The tag colon comes after the last slash
	lastSlash := strings.LastIndex(image, "/")
	colonIdx := strings.LastIndex(image, ":")

	if colonIdx == -1 || colonIdx < lastSlash {
		// No tag specified
		return image, "latest"
	}

	return image[:colonIdx], image[colonIdx+1:]
}

// stripRegistry removes known registry prefixes from an image reference.
func stripRegistry(base string) string {
	registries := []string{
		"docker.io/",
		"index.docker.io/",
		"registry.hub.docker.com/",
		"ghcr.io/",
		"quay.io/",
	}
	for _, prefix := range registries {
		if strings.HasPrefix(base, prefix) {
			return strings.TrimPrefix(base, prefix)
		}
	}

	// Handle registry with port (e.g., registry:5000/name)
	parts := strings.SplitN(base, "/", 2)
	if len(parts) == 2 && (strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":")) {
		return parts[1]
	}

	return base
}
