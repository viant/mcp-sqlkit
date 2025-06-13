package mcp

import (
    "testing"

    "github.com/stretchr/testify/assert"
)

// TestServiceUseTextField verifies that the Service decides correctly which
// field (`text` vs `data`) to populate based on configuration.  The default
// is now `text` unless UseData is set to true.
func TestServiceUseTextField(t *testing.T) {
    type testCase struct {
        name           string
        cfg            *Config
        expectText bool
    }

    testCases := []testCase{
        {
            name:       "default_text",
            cfg:        &Config{},
            expectText: true,
        },
        {
            name:       "explicit_useData",
            cfg:        &Config{UseData: true},
            expectText: false,
        },
        {
            name:       "legacy_useText",
            cfg:        &Config{UseText: true},
            expectText: true,
        },
    }

    for _, tc := range testCases {
        svc := NewService(tc.cfg)
        assert.EqualValues(t, tc.expectText, svc.UseTextField(), tc.name)
    }
}
