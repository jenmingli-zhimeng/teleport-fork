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

package events

import "sync"

// MemBuffer is an in-memory byte buffer that implements both io.Writer and
// io.WriterAt interfaces. The edgeagent variant avoids importing aws-sdk.
type MemBuffer struct {
	mu  sync.Mutex
	buf []byte
}

func (b *MemBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *MemBuffer) WriteAt(p []byte, pos int64) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	need := int(pos) + len(p)
	if need > len(b.buf) {
		nb := make([]byte, need)
		copy(nb, b.buf)
		b.buf = nb
	}
	copy(b.buf[pos:], p)
	return len(p), nil
}

func (b *MemBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]byte, len(b.buf))
	copy(out, b.buf)
	return out
}
