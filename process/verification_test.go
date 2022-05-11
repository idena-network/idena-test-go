package process

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_reverseAnswer(t *testing.T) {
	require.Equal(t, byte(0), reverseAnswer(0))
	require.Equal(t, byte(2), reverseAnswer(1))
	require.Equal(t, byte(1), reverseAnswer(2))
	require.Equal(t, byte(3), reverseAnswer(3))
}
