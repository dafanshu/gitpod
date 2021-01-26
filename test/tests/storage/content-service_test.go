// Copyright (c) 2021 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package storage

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gitpod-io/gitpod/content-service/api"
	"github.com/gitpod-io/gitpod/test/pkg/integration"
	"google.golang.org/grpc/status"
)

var (
	gitpodBuiltinUserID = "00000000-0000-0000-0000-000000000000"
)

func isErrorNotFound(err error) bool {
	st, ok := status.FromError(err)
	return ok && st.Message() == "not found"
}

func isErrorInvalidName(err error) bool {
	st, ok := status.FromError(err)
	return ok && strings.Contains(st.Message(), "needs to match regex")
}

func TestUploadUrlReturnsUrl(t *testing.T) {
	it := integration.NewTest(t)
	defer it.Done()

	bs := it.API().BlobService()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := bs.UploadUrl(ctx, &api.UploadUrlRequest{OwnerId: gitpodBuiltinUserID, Name: "test-blob"})
	if err != nil {
		t.Fatal(err)
	}
	url := resp.Url
	if url == "" {
		t.Fatal("upload url is empty")
	}
	t.Logf("Got URL repsonse: %s", url)
}

func TestUploadUrlUnknownUser(t *testing.T) {
	it := integration.NewTest(t)
	defer it.Done()

	bs := it.API().BlobService()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := bs.UploadUrl(ctx, &api.UploadUrlRequest{OwnerId: "does-not-exist", Name: "test-blob"})
	if !isErrorNotFound(err) {
		if err != nil {
			t.Fatal(err)
		} else {
			t.Fatalf("'no found' expected but error is nil, response: %+v", resp)
		}
	}
}

func TestUploadUrlInvalidName(t *testing.T) {
	it := integration.NewTest(t)
	defer it.Done()

	bs := it.API().BlobService()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := bs.UploadUrl(ctx, &api.UploadUrlRequest{OwnerId: gitpodBuiltinUserID, Name: "whitespaces are not allowed"})
	if !isErrorInvalidName(err) {
		if err != nil {
			t.Fatal(err)
		} else {
			t.Fatalf("'invalid name' expected but error is nil, response: %+v", resp)
		}
	}
}

func TestDownloadUrlReturnsNotFound(t *testing.T) {
	it := integration.NewTest(t)
	defer it.Done()

	bs := it.API().BlobService()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := bs.DownloadUrl(ctx, &api.DownloadUrlRequest{OwnerId: gitpodBuiltinUserID, Name: "does-not-exist"})
	if !isErrorNotFound(err) {
		if err != nil {
			t.Fatal(err)
		} else {
			t.Fatalf("'no found' expected but error is nil, response: %+v", resp)
		}
	}
}

func TestUploadDownloadBlob(t *testing.T) {
	it := integration.NewTest(t)
	defer it.Done()

	blobContent := fmt.Sprintf("Hello Blobs! It's %s!", time.Now())

	bs := it.API().BlobService()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := bs.UploadUrl(ctx, &api.UploadUrlRequest{OwnerId: gitpodBuiltinUserID, Name: "test-blob"})
	if err != nil {
		t.Fatal(err)
	}
	url := resp.Url
	t.Logf("upload URL: %s", url)

	uploadBlob(t, url, blobContent)

	resp2, err := bs.DownloadUrl(ctx, &api.DownloadUrlRequest{OwnerId: gitpodBuiltinUserID, Name: "test-blob"})
	if err != nil {
		t.Fatal(err)
	}
	url = resp2.Url
	t.Logf("download URL: %s", url)

	body := downloadBlob(t, url)
	if string(body) != blobContent {
		t.Fatalf("blob content mismatch: should '%s' but is '%s'", blobContent, body)
	}
}

// TestUploadDownloadBlobViaServer uploads a blob via server → content-server and downloads it afterwards
func TestUploadDownloadBlobViaServer(t *testing.T) {
	it := integration.NewTest(t)
	defer it.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	blobContent := fmt.Sprintf("Hello Blobs! It's %s!", time.Now())

	server := it.API().GitpodServer()
	url, err := server.GetContentBlobUploadURL(ctx, "test-blob")
	if err != nil {
		t.Fatalf("cannot get content blob upload URL: %q", err)
	}
	t.Logf("upload URL: %s", url)

	uploadBlob(t, url, blobContent)

	url, err = server.GetContentBlobDownloadURL(ctx, "test-blob")
	if err != nil {
		t.Fatalf("cannot get content blob download URL: %q", err)
	}
	t.Logf("download URL: %s", url)

	body := downloadBlob(t, url)
	if string(body) != blobContent {
		t.Fatalf("blob content mismatch: should '%s' but is '%s'", blobContent, body)
	}

	t.Log("Uploading and downloading blob to content store succeeded.")
}

func uploadBlob(t *testing.T, url string, content string) {
	client := &http.Client{}
	httpreq, err := http.NewRequest(http.MethodPut, url, strings.NewReader(content))
	if err != nil {
		t.Fatalf("cannot create HTTP PUT request: %q", err)
	}
	httpresp, err := client.Do(httpreq)
	if err != nil {
		t.Fatalf("HTTP PUT request failed: %q", err)
	}
	body, err := ioutil.ReadAll(httpresp.Body)
	if err != nil {
		t.Fatalf("cannot read response body of HTTP PUT: %q", err)
	}
	if string(body) != "" {
		t.Fatalf("unexpected response body of HTTP PUT: '%q'", body)
	}
}

func downloadBlob(t *testing.T, url string) string {
	httpresp, err := http.Get(url)
	if err != nil {
		t.Fatalf("HTTP GET requst failed: %q", err)
	}
	body, err := ioutil.ReadAll(httpresp.Body)
	if err != nil {
		t.Fatalf("cannot read response body of HTTP PUT: %q", err)
	}
	return string(body)
}
