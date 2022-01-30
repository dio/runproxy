// Copyright 2022 Dhi Aurrahman
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package downloader

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/bazelbuild/bazelisk/httputil"
	"github.com/codeclysm/extract"
	"github.com/ulikunitz/xz"
)

const (
	DefaultBinaryName = "envoy"
	// This is similar to: https://archive.tetratelabs.io/envoy/download/v1.12.2/envoy-v1.12.2-linux-amd64.tar.xz.
	archiveURLPattern = "https://archive.tetratelabs.io/envoy/download/v%s/envoy-v%s-%s-amd64.tar.xz"
)

// DownloadVersionedBinary returns the downloaded binary file path.
func DownloadVersionedBinary(ctx context.Context, version, destDir, destFile string) (string, error) {
	err := os.MkdirAll(destDir, 0o750)
	if err != nil {
		return "", fmt.Errorf("could not create directory %s: %v", destDir, err)
	}

	destinationPath := filepath.Join(destDir, DefaultBinaryName)
	if _, err := os.Stat(destinationPath); err != nil {
		downloadURL := GetArchiveURL(version)
		// TODO(dio): Streaming the bytes from remote file. We decided to use this for skipping copying
		// the retry logic that has already implemented in github.com/bazelbuild/bazelisk/httputil.
		data, _, err := httputil.ReadRemoteFile(downloadURL, "")
		if err != nil {
			return "", fmt.Errorf("failed to read remote file: %s: %w", downloadURL, err)
		}
		buffer := bytes.NewBuffer(data)
		streamer, err := xz.NewReader(buffer)
		if err != nil {
			return "", err
		}
		stream := io.NopCloser(streamer)
		defer stream.Close() //nolint:errcheck

		err = extract.Tar(ctx, stream, destDir, func(name string) string {
			baseName := filepath.Base(name)
			if baseName == DefaultBinaryName {
				return baseName
			}
			return name
		})
		if err != nil {
			return "", err
		}
		if _, err = os.Stat(destinationPath); err != nil {
			return "", fmt.Errorf("failed to extract the remote file from: %s: %w", downloadURL, err)
		}
		if err = os.Chmod(destinationPath, 0o755); err != nil { //nolint:gosec
			return "", fmt.Errorf("could not chmod file %s: %v", destinationPath, err)
		}
	}
	return destinationPath, nil
}

// GetArchiveURL renders the archive URL pattern to return the actual archive URL.
func GetArchiveURL(version string) string {
	return fmt.Sprintf(archiveURLPattern, version, version, runtime.GOOS) // We always do amd64, ignore the GOARCH for now.
}
