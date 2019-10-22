package process

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_getEnodeForPort(t *testing.T) {
	require.Equal(t, ":111", getEnodeForPort(":40410", 111))
	require.Equal(t, "aaa@bbb:111", getEnodeForPort("aaa@bbb:40410", 111))
}
