// Copyright (c) 2021 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package supervisor

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/bmatcuk/doublestar/v2"
	"github.com/gitpod-io/gitpod/common-go/log"
	"golang.org/x/xerrors"
)

const (
	restoreIgnoreWriteError = true
	restoreSkipExisting     = false
	compressTarball         = false
)

func backupUserConfig(url string, globs []string) error {
	piper, pipew := io.Pipe()
	go func() {
		defer pipew.Close()
		if err := createTarball(pipew, globs); err != nil {
			log.Errorf("[user config backup] error creating tarball: %v", err)
		}
	}()
	client := &http.Client{}
	httpreq, err := http.NewRequest(http.MethodPut, url, piper)
	if err != nil {
		return err
	}
	httpresp, err := client.Do(httpreq)
	if err != nil {
		return err
	}
	_, err = ioutil.ReadAll(httpresp.Body)
	if err != nil {
		return err
	}
	return nil
}

func restoreUserConfig(url string) error {
	httpresp, err := http.Get(url)
	if err != nil {
		return err
	}
	if httpresp.StatusCode != http.StatusOK {
		return xerrors.Errorf("status code of HTTP request is not OK but '%s'", httpresp.StatusCode)
	}

	var tarReader *tar.Reader
	if compressTarball {
		gzipReader, err := gzip.NewReader(httpresp.Body)
		if err != nil {
			return err
		}
		defer gzipReader.Close()
		tarReader = tar.NewReader(gzipReader)
	} else {
		tarReader = tar.NewReader(httpresp.Body)
	}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		filePath := header.Name
		perm := os.FileMode(header.Mode)
		if err := writeFile(tarReader, filePath, perm); err != nil {
			return err
		}
	}
	return nil
}

func createTarball(writer io.Writer, globs []string) error {
	var tarWriter *tar.Writer
	if compressTarball {
		gzipWriter := gzip.NewWriter(writer)
		defer gzipWriter.Close()

		tarWriter = tar.NewWriter(gzipWriter)
	} else {
		tarWriter = tar.NewWriter(writer)
	}
	defer tarWriter.Close()

	for _, glob := range globs {
		log.Debugf("[user config backup] processing glob pattern '%s'...", glob)
		filePaths, err := doublestar.Glob(glob)
		if err != nil {
			return err
		}
		for _, filePath := range filePaths {
			filePath, err := filepath.Abs(filePath)
			if err != nil {
				return err
			}
			stat, err := os.Stat(filePath)
			if err != nil {
				return err
			}
			if stat.Mode().IsRegular() { // ignore dirs
				log.Debugf("[user config backup] adding file '%s'...", filePath)
				if err := addFileToTarball(tarWriter, filePath); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func addFileToTarball(tarWriter *tar.Writer, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}
	header := &tar.Header{
		Name:    filePath,
		Size:    stat.Size(),
		Mode:    int64(stat.Mode()),
		ModTime: stat.ModTime(),
	}
	if err = tarWriter.WriteHeader(header); err != nil {
		return err
	}
	if _, err = io.Copy(tarWriter, file); err != nil {
		return err
	}
	return nil
}

func writeFile(reader io.Reader, filePath string, perm os.FileMode) error {
	log.Debugf("[user config backup] restoring file '%s' ...", filePath)
	if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		if restoreIgnoreWriteError {
			log.Errorf("[user config backup] restoring %s failed: %v", filePath, err)
			return nil
		}
		return err
	}

	flag := os.O_WRONLY | os.O_CREATE | os.O_TRUNC // create new or truncate existing file
	if restoreSkipExisting {
		flag = os.O_WRONLY | os.O_CREATE | os.O_EXCL // create new file or fail if file exists
	}
	file, err := os.OpenFile(filePath, flag, perm)
	if os.IsExist(err) {
		log.Debugf("[user config backup] skipping '%s', already exists", filePath)
		return nil
	} else if err != nil {
		if restoreIgnoreWriteError {
			log.Errorf("[user config backup] restoring %s failed: %v", filePath, err)
			return nil
		}
		return err
	}
	defer file.Close()
	if _, err = io.Copy(file, reader); err != nil {
		if restoreIgnoreWriteError {
			log.Errorf("[user config backup] restoring %s failed: %v", filePath, err)
			return nil
		}
		return err
	}
	return nil
}

func expirationFromUrl(givenURL string) (*time.Time, error) {
	u, err := url.Parse(givenURL)
	if err != nil {
		return nil, xerrors.Errorf("[user config backup] cannot parse URL '%s': %v", givenURL, err)
	}
	query := u.Query()
	d := query.Get("X-Goog-Date")
	if d == "" {
		d = query.Get("X-Amz-Date")
	}
	dd, err := time.Parse("20060102T150405Z", d)
	if err != nil {
		return nil, xerrors.Errorf("[user config backup] cannot parse X-Goog-Date/X-Amz-Date '%s': %v", d, err)
	}
	e := u.Query().Get("X-Amz-Expires")
	ee, err := strconv.Atoi(e)
	if err != nil {
		return nil, xerrors.Errorf("[user config backup] cannot parse X-Goog-Exipres/X-Amz-Expires '%s: %v", d, err)
	}
	exp := time.Duration(ee) * time.Second
	expires := dd.Add(exp)
	log.Debugf("[user config backup] url expires after %s at %s.", exp, expires)
	return &expires, nil
}

func isURLValid(url string) (bool, error) {
	expires, err := expirationFromUrl(url)
	if err != nil {
		return false, err
	}
	nowWithGracePeriod := time.Now().Add(-time.Minute)
	b := nowWithGracePeriod.Before(*expires)
	if !b {
		log.Debugf("[user config backup] now (with 1 minute grace period) '%s' is NOT before url expiration time '%s' (url is invalid)", nowWithGracePeriod, *expires)
	}
	return b, nil
}
