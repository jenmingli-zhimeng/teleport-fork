//go:build !cgo || edgeagent

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

package uacc

import (
	"net"
	"time"

	"github.com/gravitational/trace"
)

type WtmpdbBackend struct{}

func NewWtmpdbBackend(string) (*WtmpdbBackend, error) {
	return nil, trace.NotImplemented("wtmpdb is disabled in this build")
}

func (w *WtmpdbBackend) Login(string, string, net.Addr, time.Time) (int64, error) {
	return 0, trace.NotImplemented("wtmpdb is disabled in this build")
}

func (w *WtmpdbBackend) Logout(int64, time.Time) error {
	return trace.NotImplemented("wtmpdb is disabled in this build")
}

func (w *WtmpdbBackend) IsUserLoggedIn(string) (bool, error) {
	return false, trace.NotImplemented("wtmpdb is disabled in this build")
}
