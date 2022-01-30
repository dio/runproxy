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

package api

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	bootstrapv3 "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3" // added to resolved v3.HttpConnectionManager.
	"github.com/tetratelabs/run"
	"github.com/tetratelabs/telemetry"
	"google.golang.org/protobuf/encoding/protojson"
	"sigs.k8s.io/yaml"

	"github.com/dio/runproxy/internal/downloader"
	"github.com/dio/runproxy/internal/runner"
)

var (
	// Default binary version.
	DefaultBinaryVersion = "1.21.0"
	// Default download timeout.
	DefaultDownloadTimeout = 30 * time.Second
)

const (
	binaryHomeEnvKey = "PROXY_HOME"
)

// Config holds the configuration object for running auth_server.
type Config struct {
	Version string
	// Location where the binary will be downloaded.
	Dir         string
	Logger      telemetry.Logger
	ProxyConfig *bootstrapv3.Bootstrap
}

// New returns a new run.Service that wraps auth_server binary. Setting the cfg to nil, expecting
// setting the auth_server's --filter_config from a file.
func New(cfg *Config) *Service {
	if cfg == nil {
		cfg = &Config{} // TODO(dio): Have a way to generate default config.
	}
	return &Service{
		cfg: cfg,
	}
}

// Service is a run.Service implementation that runs auth_server.
type Service struct {
	cfg             *Config
	cmd             *exec.Cmd
	binaryPath      string
	configPath      string
	proxyConfigFile string
}

var _ run.Config = (*Service)(nil)

// Name returns the service name.
func (s *Service) Name() string {
	return "proxy"
}

// FlagSet provides command line flags for external auth-service.
func (s *Service) FlagSet() *run.FlagSet {
	flags := run.NewFlagSet("Proxy Service options")
	flags.StringVar(
		&s.proxyConfigFile,
		s.flagName("config"),
		s.proxyConfigFile,
		"Path to the proxy config file")

	flags.StringVar(
		&s.cfg.Version,
		s.flagName("version"),
		DefaultBinaryVersion,
		"Proxy version")

	flags.StringVar(
		&s.cfg.Dir,
		s.flagName("directory"),
		os.Getenv(binaryHomeEnvKey),
		"Path to the proxy work directory")

	return flags
}

func (s *Service) flagName(name string) string {
	return s.Name() + "-" + name
}

// Validate validates the given configuration.
func (s *Service) Validate() error {
	if s.proxyConfigFile != "" {
		b, err := os.ReadFile(s.proxyConfigFile)
		if err != nil {
			return err
		}

		// Probably a .yaml file. We simply check the extension here.
		if filepath.Ext(s.proxyConfigFile) == ".yaml" || filepath.Ext(s.proxyConfigFile) == ".yml" {
			b, err = yaml.YAMLToJSON(b)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
		}

		var cfg bootstrapv3.Bootstrap
		if err = protojson.Unmarshal(b, &cfg); err != nil {
			return err
		}
		s.cfg.ProxyConfig = &cfg
	}

	if s.cfg.ProxyConfig == nil {
		return errors.New("proxy config is required")
	}
	return s.cfg.ProxyConfig.ValidateAll()
}

// PreRun prepares the biany to run.
func (s *Service) PreRun() (err error) {
	if s.cfg.Dir == "" {
		// To make sure we have a work directory.
		dir, err := ioutil.TempDir("", downloader.DefaultBinaryName)
		if err != nil {
			return nil
		}
		s.cfg.Dir = dir
	}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultDownloadTimeout)
	defer cancel()

	// Check and download the versioned binary.
	s.binaryPath, err = downloader.DownloadVersionedBinary(ctx, s.cfg.Version, s.cfg.Dir, downloader.DefaultBinaryName)
	if err != nil {
		return err
	}

	// Generate JSON config to run the auth_server. See: authservice/docs/README.md.
	jsonConfig, err := protojson.Marshal(s.cfg.ProxyConfig)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(s.cfg.Dir, "*.json")
	if err != nil {
		return err
	}
	s.configPath = tmp.Name()

	if _, err = tmp.Write(jsonConfig); err != nil {
		return err
	}

	// TODO(dio): Allow to execute with more options.
	// Expose all envoy command line args as flags here.
	s.cmd = runner.MakeCmd(s.binaryPath, []string{"-c", s.configPath}, os.Stdout)
	return nil
}

// Serve runs the binary.
func (s *Service) Serve() error {
	// Run the downloaded auth_server with the generated config in s.configPath.
	if exitCode, err := runner.Run(s.cmd); err != nil {
		s.cfg.Logger.Error(fmt.Sprintf("%s exit with %d", downloader.DefaultBinaryName, exitCode), err)
		return err
	}
	return nil
}

// GracefulStop stops the underlying process by sending interrupt.
func (s *Service) GracefulStop() {
	if s.cmd != nil {
		s.cmd.Process.Signal(os.Interrupt)
	}
}
