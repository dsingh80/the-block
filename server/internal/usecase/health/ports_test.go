package health

import (
	"context"
	"errors"
	"testing"
)

func TestPingerFunc_AdaptsAPlainFunctionToPinger(t *testing.T) {
	t.Run("delegates to the wrapped function and returns its result", func(t *testing.T) {
		called := false
		f := PingerFunc(func(ctx context.Context) error {
			called = true
			return nil
		})

		var p Pinger = f // must genuinely satisfy the interface, not just have a matching method
		if err := p.Ping(context.Background()); err != nil {
			t.Errorf("Ping() = %v, want nil", err)
		}
		if !called {
			t.Error("the wrapped function was never invoked")
		}
	})

	t.Run("propagates an error from the wrapped function", func(t *testing.T) {
		wantErr := errors.New("unreachable")
		f := PingerFunc(func(ctx context.Context) error { return wantErr })

		if err := f.Ping(context.Background()); !errors.Is(err, wantErr) {
			t.Errorf("Ping() = %v, want %v", err, wantErr)
		}
	})
}
