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

package main

import (
	"fmt"
	"os"

	"github.com/tetratelabs/run"
	runsignal "github.com/tetratelabs/run/pkg/signal"
	"github.com/tetratelabs/telemetry"

	runproxy "github.com/dio/runproxy/api"
)

func main() {
	var (
		logger        = telemetry.NoopLogger()
		g             = &run.Group{Name: "example", Logger: logger}
		server        = runproxy.New(&runproxy.Config{Logger: logger})
		signalHandler = new(runsignal.Handler)
	)
	g.Register(server, signalHandler)
	if err := g.Run(); err != nil {
		fmt.Printf("program exit: %+v\n", err)
		os.Exit(1)
	}
}
