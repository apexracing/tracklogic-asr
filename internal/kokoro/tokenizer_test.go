package kokoro

import "testing"

func TestChunkTokensAt509Boundary(t *testing.T) {
	for _, size := range []int{1, 509, 510, 1018, 1019} {
		ids := make([]int64, size)
		for i := range ids {
			ids[i] = 10
		}
		chunks := chunkTokens(ids, 509)
		total := 0
		for _, chunk := range chunks {
			if len(chunk) == 0 || len(chunk) > 509 {
				t.Fatalf("size=%d invalid chunk length %d", size, len(chunk))
			}
			total += len(chunk)
		}
		if total != size {
			t.Fatalf("size=%d chunks contain %d tokens", size, total)
		}
	}
}

func TestChunkTokensPrefersPunctuation(t *testing.T) {
	ids := make([]int64, 700)
	for i := range ids {
		ids[i] = 10
	}
	ids[400] = 3
	chunks := chunkTokens(ids, 509)
	if len(chunks) != 2 || len(chunks[0]) != 401 {
		t.Fatalf("chunk lengths=%v, want [401 299]", []int{len(chunks[0]), len(chunks[1])})
	}
}
