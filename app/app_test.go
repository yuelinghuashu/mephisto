package app

import (
	"testing"
)

func TestRunNoLLM(t *testing.T) {
	// 注意：这只是一个“不报错”测试，真正的集成需要 Mock
	// 假设我们在 data 下有 sample.meph
	err := Run("../data/sample.meph")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}
