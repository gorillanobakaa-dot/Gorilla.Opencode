package chat

import (
	"fmt"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/message"
)

// livePreview runs on EVERY frame while a reply streams. If its cost grows with the
// length of the reply, then the longer the answer the slower the interface — which
// is exactly what "it gets sluggish" describes.
func BenchmarkLivePreviewByReplyLength(b *testing.B) {
	for _, words := range []int{50, 200, 800, 3200} {
		body := strings.Repeat("the quick brown fox jumps over the lazy dog ", words/9+1)
		b.Run(fmt.Sprintf("%dwords", words), func(b *testing.B) {
			m := &messagesCmp{width: 100, scrollback: true, printed: map[string]bool{},
				cachedContent: map[string]cacheItem{},
				messages:      []message.Message{streamingAssistant("m1", body, 1785228225)}}
			b.ReportAllocs()
			for b.Loop() {
				_ = m.livePreview()
			}
		})
	}
}
