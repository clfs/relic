// +build !windows

/*
 * Copyright (c) SAS Institute Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package audit

import (
	"os"
	"strconv"
	"syscall"
)

// Write audit record to an inherited file descriptor. This is how the
// subprocess that does the actual signing conveys audit data back to the
// server for its own logs.
func (info *AuditInfo) WriteFd() error {
	fdstr := os.Getenv(EnvAuditFd)
	if fdstr == "" {
		return nil
	}
	blob, err := info.Marshal()
	if err != nil {
		return err
	}
	fd, err := strconv.Atoi(fdstr)
	if err != nil {
		return err
	}
	newfd, err := syscall.Dup(fd)
	if err != nil {
		return err
	}
	af := os.NewFile(uintptr(newfd), "<audit>")
	defer af.Close()
	_, err = af.Write(blob)
	return err
}
