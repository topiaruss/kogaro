package adaptive

import (
	"testing"
)

func TestLookupImage_ExactMatch(t *testing.T) {
	traits, ok := LookupImage("nginx")
	if !ok {
		t.Fatal("expected to find nginx")
	}
	if traits.DefaultUID != 101 {
		t.Errorf("expected UID 101, got %d", traits.DefaultUID)
	}
}

func TestLookupImage_RegistryPrefix(t *testing.T) {
	traits, ok := LookupImage("docker.io/library/nginx")
	if !ok {
		t.Fatal("expected to find docker.io/library/nginx")
	}
	if traits.DefaultUID != 101 {
		t.Errorf("expected UID 101, got %d", traits.DefaultUID)
	}
}

func TestLookupImage_WithTag(t *testing.T) {
	traits, ok := LookupImage("nginx:1.25-alpine")
	if !ok {
		t.Fatal("expected to find nginx:1.25-alpine")
	}
	if traits.DefaultUID != 101 {
		t.Errorf("expected UID 101, got %d", traits.DefaultUID)
	}
}

func TestLookupImage_BitnamiPrefix(t *testing.T) {
	traits, ok := LookupImage("bitnami/postgresql")
	if !ok {
		t.Fatal("expected to find bitnami/postgresql")
	}
	if traits.DefaultUID != 1001 {
		t.Errorf("expected UID 1001, got %d", traits.DefaultUID)
	}
	if traits.EnvPrefix != "POSTGRESQL_" {
		t.Errorf("expected env prefix POSTGRESQL_, got %s", traits.EnvPrefix)
	}
}

func TestLookupImage_BitnamiWithRegistry(t *testing.T) {
	traits, ok := LookupImage("docker.io/bitnami/postgresql:15.2")
	if !ok {
		t.Fatal("expected to find docker.io/bitnami/postgresql:15.2")
	}
	if traits.DefaultUID != 1001 {
		t.Errorf("expected UID 1001, got %d", traits.DefaultUID)
	}
}

func TestLookupImage_Distroless(t *testing.T) {
	traits, ok := LookupImage("gcr.io/distroless/static:nonroot")
	if !ok {
		t.Fatal("expected to find gcr.io/distroless/static:nonroot")
	}
	if !traits.IsDistroless {
		t.Error("expected IsDistroless=true")
	}
	if !traits.SafeForReadOnlyFS {
		t.Error("expected SafeForReadOnlyFS=true")
	}
}

func TestLookupImage_Unknown(t *testing.T) {
	_, ok := LookupImage("my-company/custom-app:v2.3.1")
	if ok {
		t.Error("expected unknown image to return false")
	}
}

func TestParseImageRef_Simple(t *testing.T) {
	base, tag := ParseImageRef("nginx")
	if base != "nginx" || tag != "latest" {
		t.Errorf("expected (nginx, latest), got (%s, %s)", base, tag)
	}
}

func TestParseImageRef_WithTag(t *testing.T) {
	base, tag := ParseImageRef("nginx:1.25-alpine")
	if base != "nginx" || tag != "1.25-alpine" {
		t.Errorf("expected (nginx, 1.25-alpine), got (%s, %s)", base, tag)
	}
}

func TestParseImageRef_RegistryWithPort(t *testing.T) {
	base, tag := ParseImageRef("registry.example.com:5000/myapp:v1.0")
	if base != "registry.example.com:5000/myapp" || tag != "v1.0" {
		t.Errorf("expected (registry.example.com:5000/myapp, v1.0), got (%s, %s)", base, tag)
	}
}

func TestParseImageRef_RegistryNoTag(t *testing.T) {
	base, tag := ParseImageRef("registry.example.com:5000/myapp")
	if base != "registry.example.com:5000/myapp" || tag != "latest" {
		t.Errorf("expected (registry.example.com:5000/myapp, latest), got (%s, %s)", base, tag)
	}
}

func TestParseImageRef_Digest(t *testing.T) {
	base, tag := ParseImageRef("nginx@sha256:abc123")
	if base != "nginx" || tag != "sha256:abc123" {
		t.Errorf("expected (nginx, sha256:abc123), got (%s, %s)", base, tag)
	}
}

func TestParseImageRef_FullRegistryPath(t *testing.T) {
	base, tag := ParseImageRef("gcr.io/my-project/my-app:v2.0")
	if base != "gcr.io/my-project/my-app" || tag != "v2.0" {
		t.Errorf("expected (gcr.io/my-project/my-app, v2.0), got (%s, %s)", base, tag)
	}
}
