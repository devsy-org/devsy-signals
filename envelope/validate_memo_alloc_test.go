//go:build !race

package envelope_test

import "testing"

func TestValidatePointerChainAllocations(t *testing.T) {
	t.Parallel()
	e := pointerChainRecord(t)
	result := testing.Benchmark(func(b *testing.B) {
		b.Helper()
		benchmarkData(b, e)
	})
	if allocated := result.AllocedBytesPerOp(); allocated > 512<<10 {
		t.Errorf("Validate allocated %d bytes/op, want at most 512 KiB", allocated)
	}
}
