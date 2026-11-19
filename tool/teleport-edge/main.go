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
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/edgeagent"
	"github.com/gravitational/teleport/session/reexec"
)

func main() {
	reexec.MaybeReexec()

	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "teleport-edge: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("expected a command (start or version)")
	}
	switch args[0] {
	case "version", "--version", "-version":
		printVersion()
		return nil
	case "start":
		return start(args[1:])
	default:
		return fmt.Errorf("unknown command %q (want start or version)", args[0])
	}
}

func start(args []string) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	configPath := fs.String("config", "/etc/teleport.yaml", "path to teleport-edge yaml config")
	insecure := fs.Bool("insecure", false, "skip TLS verification (development only)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := edgeagent.LoadFile(*configPath)
	if err != nil {
		return err
	}
	if *insecure {
		cfg.Teleport.Insecure = true
		lib.SetInsecureDevMode(true)
	}
	return edgeagent.Run(context.Background(), cfg)
}

func printVersion() {
	fmt.Printf("Teleport Edge v%s git:%s go%s\n", teleport.Version, teleport.Gitref, runtime.Version())
}
