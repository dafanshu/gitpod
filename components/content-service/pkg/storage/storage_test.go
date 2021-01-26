// Copyright (c) 2021 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package storage

import (
	"fmt"
	"strings"
	"testing"
)

func TestBlobObjectName(t *testing.T) {
	name := "my-object-name"
	objectName, err := blobObjectName(name)
	if err != nil {
		t.Fatalf("%+v", err)
	}
	exptectedObjectName := fmt.Sprintf("blobs/%s", name)
	if objectName != exptectedObjectName {
		t.Fatalf("unexpected object name: is '%s' but expected '%s'", objectName, exptectedObjectName)
	}
}

func TestBlobObjectNameWithSlash(t *testing.T) {
	name := "my-object-name/with-slash"
	objectName, err := blobObjectName(name)
	if err != nil {
		t.Fatalf("%+v", err)
	}
	exptectedObjectName := fmt.Sprintf("blobs/%s", name)
	if objectName != exptectedObjectName {
		t.Fatalf("unexpected object name: is '%s' but expected '%s'", objectName, exptectedObjectName)
	}
}

func TestBlobObjectNameWithWhitespace(t *testing.T) {
	name := "object name with whitespace"
	_, err := blobObjectName(name)
	if err == nil {
		t.Fatal("blob name with whitespace should be rejected")
	}
	if !strings.Contains(err.Error(), "needs to match regex") {
		t.Fatalf("%+v", err)
	}
}

func TestBlobObjectNameWithInvalidChar(t *testing.T) {
	name := "object-name-with-invälid-char"
	_, err := blobObjectName(name)
	if err == nil {
		t.Fatal("blob name with invalid char should be rejected")
	}
	if !strings.Contains(err.Error(), "needs to match regex") {
		t.Fatalf("%+v", err)
	}
}
