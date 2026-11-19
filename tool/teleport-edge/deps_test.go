//go:build edgeagent

// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

// forbiddenImportPrefixes must not appear in the teleport-edge dependency
// graph. Keep this list in sync with lib/edgeagent/PATCHES.md.
//
// aws-sdk-go-v2/aws/arn is used for IAM role ARN parsing until that helper
// is split out of lib/services.
var forbiddenImportPrefixes = []string{
	"github.com/gravitational/teleport/lib/web",
	"github.com/gravitational/teleport/lib/kube",
	"github.com/gravitational/teleport/lib/cloud",
	"github.com/gravitational/teleport/lib/desktop",
	"github.com/gravitational/teleport/lib/auth/join",
	"github.com/gravitational/teleport/lib/proxy/peer",
	"github.com/gravitational/teleport/lib/srv/db",
	"github.com/mattn/go-sqlite3",
	"github.com/quic-go/quic-go",
	"github.com/crewjam/saml",
	"github.com/russellhaering/gosaml2",
	"go.mongodb.org/mongo-driver",
	"k8s.io",
	"github.com/Azure",
	"cloud.google.com",
	"github.com/go-webauthn/webauthn",
	"github.com/google/go-tpm",
	"github.com/sigstore",
	"github.com/theupdateframework/go-tuf",
	"github.com/opencontainers/selinux",
	"github.com/schollz/progressbar",
	"github.com/gravitational/teleport/lib/connectmycomputer",
}

var forbiddenExact = []string{
	"github.com/gravitational/teleport/lib/auth",
	// lib/proxy is the SSH router (pulls desktop).
	"github.com/gravitational/teleport/lib/proxy",
}

func TestEdgeAgentDependencyBlacklist(t *testing.T) {
	cmd := exec.Command("go", "list", "-e", "-tags=edgeagent", "-deps", "-f", "{{if not .Standard}}{{.ImportPath}}{{end}}", ".")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go list: %v\n%s", err, stderr.String())
	}
	var hits []string
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, exact := range forbiddenExact {
			if line == exact {
				hits = append(hits, line)
			}
		}
		for _, prefix := range forbiddenImportPrefixes {
			if line == prefix || strings.HasPrefix(line, prefix+"/") {
				hits = append(hits, line)
			}
		}
		if strings.Contains(line, "aws-sdk-go-v2") && !strings.HasPrefix(line, "github.com/aws/aws-sdk-go-v2/aws/arn") {
			hits = append(hits, line)
		}
	}
	if len(hits) > 0 {
		t.Fatalf("teleport-edge still depends on forbidden packages:\n  %s", strings.Join(hits, "\n  "))
	}
}
