package routertraffic

import (
	"context"
	"testing"

	"github.com/jaxxstorm/starlight/satmux"
)

func TestCoreOpenerReturnsNilInterfaceOnError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client, err := satmux.NewClient(ctx, "127.0.0.1:1", []byte("key"), 1)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	defer client.Close()
	stream, err := CoreOpener(client)(context.Background(), "192.0.2.1", 443)
	if err == nil || stream != nil {
		t.Fatalf("stream=%v error=%v", stream, err)
	}
}
