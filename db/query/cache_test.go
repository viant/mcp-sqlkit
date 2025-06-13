package query

import (
    "reflect"
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestRecordTypeCacheLRU(t *testing.T) {
    type testCase struct {
        capacity int
        keys      []string
        wantExist map[string]bool
    }

    testCases := []testCase{
        {
            capacity: 2,
            keys:      []string{"a", "b", "a", "c"}, // access order leads to eviction of "b"
            wantExist: map[string]bool{"a": true, "b": false, "c": true},
        },
        {
            capacity: 1,
            keys:      []string{"x", "y"},
            wantExist: map[string]bool{"x": false, "y": true},
        },
    }

    dummyType := reflect.TypeOf(struct{}{})

    for _, tc := range testCases {
        c := newRecordTypeCache(tc.capacity)
        for _, k := range tc.keys {
            if _, ok := c.Get(k); !ok {
                c.Put(k, dummyType)
            }
        }

        for key, expected := range tc.wantExist {
            _, ok := c.Get(key)
            assert.EqualValues(t, expected, ok, "unexpected cache presence for key %s", key)
        }
    }
}
