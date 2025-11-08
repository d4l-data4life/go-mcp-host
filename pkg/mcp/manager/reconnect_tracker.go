package manager

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/d4l-data4life/go-svc/pkg/logging"
)

const listenFailurePrefix = "failed to listen to server."

type reconnectTracker struct {
	manager      *Manager
	conversation uuid.UUID
	serverName   string
	maxAttempts  int
	delay        time.Duration
	closeOnce    sync.Once
	failureCount int
	mu           sync.Mutex
	closed       atomic.Bool
	done         chan struct{}
}

func newReconnectTracker(
	manager *Manager,
	conversationID uuid.UUID,
	serverName string,
) *reconnectTracker {
	delay := manager.reconnectDelay
	if delay <= 0 {
		delay = defaultListenRetryInterval
	}

	return &reconnectTracker{
		manager:      manager,
		conversation: conversationID,
		serverName:   serverName,
		maxAttempts:  manager.maxReconnectAttempts,
		delay:        delay,
		done:         make(chan struct{}),
	}
}

func (t *reconnectTracker) markClosed() {
	if t == nil {
		return
	}
	t.closed.Store(true)
	select {
	case <-t.done:
		return
	default:
		close(t.done)
	}
}

func (t *reconnectTracker) handleListenFailure(message string) {
	if t == nil || t.closed.Load() {
		return
	}

	t.applyDelay()

	if t.closed.Load() {
		return
	}

	attempt := t.incrementFailureCount()
	if t.maxAttempts > 0 && attempt >= t.maxAttempts {
		t.closeOnce.Do(func() {
			logging.LogWarningf(nil,
				"MCP server %s unreachable (%d/%d) for conversation=%s; closing session",
				t.serverName,
				attempt,
				t.maxAttempts,
				t.conversation,
			)
			go func() {
				if err := t.manager.CloseSession(t.conversation, t.serverName); err != nil {
					logging.LogErrorf(err,
						"Failed to close MCP session after reaching retry limit: conversation=%s server=%s",
						t.conversation,
						t.serverName,
					)
				}
			}()
		})
		return
	}

	if t.maxAttempts > 0 {
		logging.LogWarningf(nil,
			"MCP server %s listen failure (%d/%d): %s",
			t.serverName,
			attempt,
			t.maxAttempts,
			message,
		)
	} else {
		logging.LogWarningf(nil,
			"MCP server %s listen failure: %s",
			t.serverName,
			message,
		)
	}
}

func (t *reconnectTracker) incrementFailureCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.failureCount++
	return t.failureCount
}

func (t *reconnectTracker) applyDelay() {
	extra := t.delay - defaultListenRetryInterval
	if extra <= 0 {
		return
	}

	timer := time.NewTimer(extra)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-t.done:
	}
}

type transportLogger struct {
	serverName string
	tracker    *reconnectTracker
}

func (l *transportLogger) Infof(format string, v ...any) {
	logging.LogInfof("[mcp:%s] %s", l.serverName, fmt.Sprintf(format, v...))
}

func (l *transportLogger) Errorf(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	if l.tracker != nil && strings.HasPrefix(format, listenFailurePrefix) {
		l.tracker.handleListenFailure(msg)
		return
	}

	logging.LogErrorf(errors.New(msg), "[mcp:%s] %s", l.serverName, msg)
}
