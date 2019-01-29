package amqp_kit

import "testing"

type matchResult struct {
	Key  string
	Mask string
	Res  bool
}

func TestMatcher(t *testing.T) {

	testKey := "foo.bar.baz"

	tests := []matchResult{
		{testKey, "foo.*.biz", false},
		{testKey, "foo.bur.*", false},
		{testKey, "foo.*.*.*", false},
		{testKey, "foo.#", true},
		{testKey, testKey, true},
		{testKey, "*.bar.*", true},
		{testKey, "#", true},
		{testKey, "*.*.baz", true},
	}

	for _, tt := range tests {
		if match(tt.Key, tt.Mask) != tt.Res {
			t.Fatalf("match(%s, %s) != %t", tt.Key, tt.Mask, tt.Res)
		}
	}
}
