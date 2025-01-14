package tracing_test

import (
	"context"
	"testing"

	"github.com/mailgun/holster/v4/tracing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestDummySpan(t *testing.T) {
	ctx := context.Background()
	withErrorAttr := trace.WithAttributes(attribute.Bool("error", true))

	t.Run("Single dropped span", func(t *testing.T) {
		// Mock OTel exporter.
		mockProcessor := new(MockSpanProcessor)
		mockProcessor.On("Shutdown", mock.Anything).Once().Return(nil)

		level := tracing.InfoLevel
		setupMockTracerProvider(t, level, mockProcessor)

		// Call code.
		ctx1 := tracing.StartNamedScopeDebug(ctx, t.Name())
		tracing.EndScope(ctx1, nil)

		err := tracing.CloseTracing(ctx)
		require.NoError(t, err)

		// Verify.
		mockProcessor.AssertExpectations(t)
	})

	t.Run("Nested scope with dropped leaf span", func(t *testing.T) {
		// Mock OTel exporter.
		mockProcessor := new(MockSpanProcessor)
		matchFirstSpan := mock.MatchedBy(func(s sdktrace.ReadOnlySpan) bool {
			return s.Name() == t.Name()
		})
		mockProcessor.On("OnStart", mock.Anything, matchFirstSpan).Once()
		mockProcessor.On("OnEnd", matchFirstSpan).
			Once().
			Run(func(args mock.Arguments) {
				s := args.Get(0).(sdktrace.ReadOnlySpan)
				assertReadOnlySpanNoError(t, s)
				assertHasLogLevel(t, tracing.InfoLevel, s)
			})
		mockProcessor.On("Shutdown", mock.Anything).Once().Return(nil)

		level := tracing.InfoLevel
		setupMockTracerProvider(t, level, mockProcessor)

		// Call code.
		ctx1 := tracing.StartNamedScopeInfo(ctx, t.Name())
		ctx2 := tracing.StartNamedScopeDebug(ctx1, "Level 2 leaf dropped", withErrorAttr)
		tracing.EndScope(ctx2, nil)
		tracing.EndScope(ctx1, nil)

		err := tracing.CloseTracing(ctx)
		require.NoError(t, err)

		// Verify.
		mockProcessor.AssertExpectations(t)
	})

	t.Run("Nested scopes with interleaved dropped span", func(t *testing.T) {
		// Mock OTel exporter.
		mockProcessor := new(MockSpanProcessor)
		matchFirstSpan := mock.MatchedBy(func(s sdktrace.ReadOnlySpan) bool {
			return s.Name() == "Level 1"
		})
		matchLeafSpan := mock.MatchedBy(func(s sdktrace.ReadOnlySpan) bool {
			return s.Name() == "Leaf"
		})
		mockProcessor.On("OnStart", mock.Anything, matchFirstSpan).Once()
		mockProcessor.On("OnStart", mock.Anything, matchLeafSpan).Once()
		var firstSpan, leafSpan sdktrace.ReadOnlySpan
		mockProcessor.On("OnEnd", matchFirstSpan).
			Once().
			Run(func(args mock.Arguments) {
				s := args.Get(0).(sdktrace.ReadOnlySpan)
				assertReadOnlySpanNoError(t, s)
				assertHasLogLevel(t, tracing.InfoLevel, s)
				firstSpan = s
			})
		mockProcessor.On("OnEnd", matchLeafSpan).
			Once().
			Run(func(args mock.Arguments) {
				s := args.Get(0).(sdktrace.ReadOnlySpan)
				assertReadOnlySpanNoError(t, s)
				assertHasLogLevel(t, tracing.InfoLevel, s)
				leafSpan = s
			})
		mockProcessor.On("Shutdown", mock.Anything).Once().Return(nil)

		level := tracing.InfoLevel
		setupMockTracerProvider(t, level, mockProcessor)

		// Call code.
		ctx1 := tracing.StartNamedScopeInfo(ctx, "Level 1")
		ctx2 := tracing.StartNamedScopeDebug(ctx1, "Level 2 dropped", withErrorAttr)
		ctx3 := tracing.StartNamedScopeInfo(ctx2, "Leaf")
		tracing.EndScope(ctx3, nil)
		tracing.EndScope(ctx2, nil)
		tracing.EndScope(ctx1, nil)

		err := tracing.CloseTracing(ctx)
		require.NoError(t, err)

		// Verify.
		mockProcessor.AssertExpectations(t)
		// Assert spans are linked: first -> leaf.
		assert.Equal(t, firstSpan.SpanContext().SpanID(), leafSpan.Parent().SpanID())
	})

	t.Run("Nested scopes with multiple dropped leaf spans", func(t *testing.T) {
		// Mock OTel exporter.
		mockProcessor := new(MockSpanProcessor)
		matchFirstSpan := mock.MatchedBy(func(s sdktrace.ReadOnlySpan) bool {
			return s.Name() == "Level 1"
		})
		mockProcessor.On("OnStart", mock.Anything, matchFirstSpan).Once()
		mockProcessor.On("OnEnd", matchFirstSpan).
			Once().
			Run(func(args mock.Arguments) {
				s := args.Get(0).(sdktrace.ReadOnlySpan)
				assertReadOnlySpanNoError(t, s)
				assertHasLogLevel(t, tracing.InfoLevel, s)
			})
		mockProcessor.On("Shutdown", mock.Anything).Once().Return(nil)

		level := tracing.InfoLevel
		setupMockTracerProvider(t, level, mockProcessor)

		// Call code.
		ctx1 := tracing.StartNamedScopeInfo(ctx, "Level 1")
		ctx2 := tracing.StartNamedScopeDebug(ctx1, "Level 2 dropped", withErrorAttr)
		ctx3 := tracing.StartNamedScopeDebug(ctx2, "Level 3 dropped", withErrorAttr)
		ctx4 := tracing.StartNamedScopeDebug(ctx3, "leaf dropped", withErrorAttr)
		tracing.EndScope(ctx4, nil)
		tracing.EndScope(ctx3, nil)
		tracing.EndScope(ctx2, nil)
		tracing.EndScope(ctx1, nil)

		err := tracing.CloseTracing(ctx)
		require.NoError(t, err)

		// Verify.
		mockProcessor.AssertExpectations(t)
	})

	t.Run("Nested scopes with multiple interleaved dropped spans", func(t *testing.T) {
		// Mock OTel exporter.
		mockProcessor := new(MockSpanProcessor)
		matchFirstSpan := mock.MatchedBy(func(s sdktrace.ReadOnlySpan) bool {
			return s.Name() == "Level 1"
		})
		matchLeafSpan := mock.MatchedBy(func(s sdktrace.ReadOnlySpan) bool {
			return s.Name() == "Leaf"
		})
		mockProcessor.On("OnStart", mock.Anything, matchFirstSpan).Once()
		mockProcessor.On("OnStart", mock.Anything, matchLeafSpan).Once()
		var firstSpan, leafSpan sdktrace.ReadOnlySpan
		mockProcessor.On("OnEnd", matchFirstSpan).
			Once().
			Run(func(args mock.Arguments) {
				s := args.Get(0).(sdktrace.ReadOnlySpan)
				assertReadOnlySpanNoError(t, s)
				assertHasLogLevel(t, tracing.InfoLevel, s)
				firstSpan = s
			})
		mockProcessor.On("OnEnd", matchLeafSpan).
			Once().
			Run(func(args mock.Arguments) {
				s := args.Get(0).(sdktrace.ReadOnlySpan)
				assertReadOnlySpanNoError(t, s)
				assertHasLogLevel(t, tracing.InfoLevel, s)
				leafSpan = s
			})
		mockProcessor.On("Shutdown", mock.Anything).Once().Return(nil)

		level := tracing.InfoLevel
		setupMockTracerProvider(t, level, mockProcessor)

		// Call code.
		ctx1 := tracing.StartNamedScopeInfo(ctx, "Level 1")
		ctx2 := tracing.StartNamedScopeDebug(ctx1, "Level 2 dropped", withErrorAttr)
		ctx3 := tracing.StartNamedScopeDebug(ctx2, "Level 3 dropped", withErrorAttr)
		ctx4 := tracing.StartNamedScopeDebug(ctx3, "Level 4 dropped", withErrorAttr)
		ctx5 := tracing.StartNamedScopeInfo(ctx4, "Leaf")
		tracing.EndScope(ctx5, nil)
		tracing.EndScope(ctx4, nil)
		tracing.EndScope(ctx3, nil)
		tracing.EndScope(ctx2, nil)
		tracing.EndScope(ctx1, nil)

		err := tracing.CloseTracing(ctx)
		require.NoError(t, err)

		// Verify.
		mockProcessor.AssertExpectations(t)
		// Assert spans are linked: first -> leaf.
		assert.Equal(t, firstSpan.SpanContext().SpanID(), leafSpan.Parent().SpanID())
	})
}

func assertHasLogLevel(t *testing.T, expectedLogLevel tracing.Level, s sdktrace.ReadOnlySpan) {
	level, ok := levelFromReadOnlySpan(s)
	if !ok {
		t.Error("Error: Expected span log level to be defined")
		return
	}

	assert.Equal(t, expectedLogLevel, level, "Span log level mismatch")
}

func assertReadOnlySpanNoError(t *testing.T, s sdktrace.ReadOnlySpan) {
	for _, attr := range s.Attributes() {
		if string(attr.Key) == "error" {
			assert.True(t, attr.Value.AsBool())
		}
	}
}

func levelFromReadOnlySpan(s sdktrace.ReadOnlySpan) (tracing.Level, bool) {
	for _, attr := range s.Attributes() {
		if string(attr.Key) == tracing.LogLevelNumKey {
			return tracing.Level(attr.Value.AsInt64()), true
		}
	}

	return tracing.Level(0), false
}

func setupMockTracerProvider(t *testing.T, level tracing.Level, mockProcessor *MockSpanProcessor) {
	t.Setenv("OTEL_EXPORTERS", "none")
	ctx := context.Background()
	opts := []tracing.TracingOption{
		tracing.WithTracerProviderOption(sdktrace.WithSpanProcessor(mockProcessor)),
		tracing.WithLevel(level),
	}
	err := tracing.InitTracing(ctx, "foobar", opts...)
	require.NoError(t, err)
}
