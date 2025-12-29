package gitops

import (
	"testing"
)

func TestParseApplication(t *testing.T) {
	yaml := `
apiVersion: zerverless.io/v1
kind: Application
metadata:
  name: test-app
  namespace: testuser
spec:
  source:
    repoURL: https://github.com/user/repo
    branch: main
  functions:
    - path: /hello
      runtime: lua
      code: |
        function handle(req)
          return {status=200, body="Hello"}
        end
  static:
    dir: ./static
`

	app, err := ParseApplication([]byte(yaml))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if app.Metadata.Name != "test-app" {
		t.Errorf("expected name test-app, got %s", app.Metadata.Name)
	}

	if app.Metadata.Namespace != "testuser" {
		t.Errorf("expected namespace testuser, got %s", app.Metadata.Namespace)
	}

	if app.Spec.Source.Branch != "main" {
		t.Errorf("expected branch main, got %s", app.Spec.Source.Branch)
	}

	if len(app.Spec.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(app.Spec.Functions))
	}

	fn := app.Spec.Functions[0]
	if fn.Path != "/hello" {
		t.Errorf("expected path /hello, got %s", fn.Path)
	}

	if fn.Runtime != "lua" {
		t.Errorf("expected runtime lua, got %s", fn.Runtime)
	}

	if app.Spec.Static == nil {
		t.Fatal("expected static config")
	}

	if app.Spec.Static.Dir != "./static" {
		t.Errorf("expected static dir ./static, got %s", app.Spec.Static.Dir)
	}
}

func TestParseApplicationWithDocker(t *testing.T) {
	yaml := `
apiVersion: zerverless.io/v1
kind: Application
metadata:
  name: docker-app
  namespace: testuser
spec:
  source:
    repoURL: https://github.com/user/repo
    branch: main
  functions:
    - path: /docker
      runtime: docker
      dockerfile: ./docker/Dockerfile
      context: ./docker
`

	app, err := ParseApplication([]byte(yaml))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(app.Spec.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(app.Spec.Functions))
	}

	fn := app.Spec.Functions[0]
	if fn.Runtime != "docker" {
		t.Errorf("expected runtime docker, got %s", fn.Runtime)
	}

	if fn.Dockerfile != "./docker/Dockerfile" {
		t.Errorf("expected dockerfile ./docker/Dockerfile, got %s", fn.Dockerfile)
	}

	if fn.Context != "./docker" {
		t.Errorf("expected context ./docker, got %s", fn.Context)
	}
}

