# Copyright 2022 Dhi Aurrahman
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Include versions of tools we build or fetch on-demand.
include Tools.mk

# Root dir returns absolute path of current directory. It has a trailing "/".
root_dir := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# Currently we resolve it using which. But more sophisticated approach is to use infer GOROOT.
go     := $(shell which go)
goarch := $(shell $(go) env GOARCH)
goexe  := $(shell $(go) env GOEXE)
goos   := $(shell $(go) env GOOS)

# Local cache directory.
CACHE_DIR ?= $(root_dir).cache

# Go tools directory holds the binaries of Go-based tools.
go_tools_dir := $(CACHE_DIR)/tools/go

# Go-based tools.
addlicense          := $(go_tools_dir)/addlicense
golangci-lint       := $(go_tools_dir)/golangci-lint
goimports           := $(go_tools_dir)/goimports

# By default, unless GOMAXPROCS is set via an environment variable or explicity in the code, the
# tests are run with GOMAXPROCS=1. This is problematic if the tests require more than one CPU, for
# example when running t.Parallel() in tests.
export GOMAXPROCS ?=4
test: ## Run all unit tests
	@$(go) test ./internal/...

check: # Make sure we follow the rules
	@rm -fr generated
	@$(MAKE) format lint license
	@if [ ! -z "`git status -s`" ]; then \
		echo "The following differences will fail CI until committed:"; \
		git diff --exit-code; \
	fi

license_ignore :=
license_files  := api example internal buf.*.yaml
license: $(addlicense)
	@$(addlicense) $(license_ignore) -c "Dhi Aurrahman"  $(license_files) 1>/dev/null 2>&1

all_nongen_go_sources := $(wildcard api/*.go example/*.go internal/*.go internal/*/*.go internal/*/*/*.go)
format: go.mod $(all_nongen_go_sources) $(goimports)
	@$(go) mod tidy
	@$(go)fmt -s -w $(all_nongen_go_sources)
# Workaround inconsistent goimports grouping with awk until golang/go#20818 or incu6us/goimports-reviser#50
	@for f in $(all_nongen_go_sources); do \
			awk '/^import \($$/,/^\)$$/{if($$0=="")next}{print}' $$f > /tmp/fmt; \
	    mv /tmp/fmt $$f; \
	done
	@$(goimports) -local $$(sed -ne 's/^module //gp' go.mod) -w $(all_nongen_go_sources)

# Override lint cache directory. https://golangci-lint.run/usage/configuration/#cache.
export GOLANGCI_LINT_CACHE=$(CACHE_DIR)/golangci-lint
lint: .golangci.yml $(all_nongen_go_sources) $(golangci-lint)
	@printf "$(ansi_format_dark)" $@ "linting Go files..."
	@$(golangci-lint) run --timeout 5m --config $< ./...
	@printf "$(ansi_format_bright)" $@ "ok"

# Catch all rules for Go-based tools.
$(go_tools_dir)/%:
	@GOBIN=$(go_tools_dir) go install $($(notdir $@)@v)
