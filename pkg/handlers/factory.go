package handlers

import (
	"sync"

	"github.com/gesundheitscloud/go-svc/pkg/instrumented"
)

var once sync.Once

var instance *instrumented.HandlerFactory

// GetHandlerFactory returns a global singleton InstrumentedHandlerFactory object
func GetHandlerFactory() *instrumented.HandlerFactory {
	once.Do(func() {
		instance = instrumented.NewHandlerFactory(
			"d4l",
			instrumented.DefaultInstrumentInitOptions,
			instrumented.DefaultInstrumentOptions,
		)
	})
	return instance
}
