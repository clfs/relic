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

package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

const HealthCheckInterval = time.Second * 60
const HealthCheckMaxFailures = 2
const PingTimeout = time.Second * 15

var (
	healthStatus   int = HealthCheckMaxFailures
	healthLastPing time.Time
	healthMu       sync.Mutex
)

func (s *Server) startHealthCheck(force bool) error {
	if !s.healthCheck() && !force && !s.Config.Server.Disabled {
		return errors.New("health check failed")
	}
	go s.healthCheckLoop()
	return nil
}

func (s *Server) healthCheckLoop() {
	t := time.NewTimer(HealthCheckInterval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			s.healthCheck()
			t.Reset(HealthCheckInterval)
		case <-s.Closed:
			break
		}
	}
}

func (s *Server) healthCheck() bool {
	healthMu.Lock()
	last := healthStatus
	healthMu.Unlock()
	ok := true
	sawToken := make(map[string]bool)
	for _, keyConf := range s.Config.Keys {
		token := keyConf.Token
		if token == "" || len(keyConf.Roles) == 0 || sawToken[token] {
			continue
		}
		sawToken[token] = true
		if !s.pingOne(token) {
			ok = false
			break
		}
	}
	next := last
	if s.Config.Server.Disabled {
		if last != 0 {
			s.Logf("server is disabled by configuration")
		}
		next = 0
	} else if ok {
		if last == 0 {
			s.Logf("recovered to normal state, status is now OK")
		} else if last < HealthCheckMaxFailures {
			s.Logf("recovered to normal state")
		}
		next = HealthCheckMaxFailures
	} else if last > 0 {
		next--
		if next == 0 {
			s.Logf("exceeded maximum health check failures, flagging as ERROR")
		}
	}
	healthMu.Lock()
	defer healthMu.Unlock()
	healthStatus = next
	healthLastPing = time.Now()
	return ok
}

func (s *Server) pingOne(tokenName string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), PingTimeout)
	defer cancel()
	var output bytes.Buffer
	proc := exec.CommandContext(ctx, os.Args[0], "ping", "--config", s.Config.Path(), "--token", tokenName)
	proc.Stdout = &output
	proc.Stderr = &output
	err := proc.Run()
	if err == nil {
		return true
	}
	select {
	case <-ctx.Done():
		s.Logf("error: health check of token %s timed out", tokenName)
	default:
		s.Logf("error: health check of token %s failed: %s\n%s\n", tokenName, err, output.String())
	}
	return false
}

func (s *Server) Healthy(request *http.Request) bool {
	healthMu.Lock()
	defer healthMu.Unlock()
	if time.Since(healthLastPing) > 3*HealthCheckInterval {
		if request != nil {
			s.Logr(request, "error: health check AWOL for %d seconds", time.Since(healthLastPing)/time.Second)
		}
		return false
	} else {
		return healthStatus > 0
	}
}

func (s *Server) serveHealth(request *http.Request) (res Response, err error) {
	if request.Method != "GET" {
		return ErrorResponse(http.StatusMethodNotAllowed), nil
	}
	if s.Healthy(request) {
		return StringResponse(http.StatusOK, "OK"), nil
	} else {
		return ErrorResponse(http.StatusServiceUnavailable), nil
	}
}
