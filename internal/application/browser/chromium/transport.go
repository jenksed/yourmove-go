package chromium

import (
	"context"
	"fmt"

	"github.com/coder/websocket"
)

// Chromium DevTools JSON messages can exceed the websocket library's default
// 32 KiB limit on realistic ATS pages. Keep an explicit finite upper bound;
// do not turn off message limits on a local debugging endpoint.
const maxCDPMessageBytes int64 = 2 << 20

func dialCDP(ctx context.Context, endpoint string) (*websocket.Conn, error) {
	ws, _, err := websocket.Dial(ctx, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to local Chromium DevTools: %w", err)
	}
	ws.SetReadLimit(maxCDPMessageBytes)
	return ws, nil
}
