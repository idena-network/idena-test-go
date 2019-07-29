package scenario

import (
	"github.com/stretchr/testify/require"
	"testing"
)

/*func TestAnswerRates_Get(t *testing.T) {
	ar := AnswerRates{
		Wrong:         0.3,
		Inappropriate: 0.2,
		None:          0.1,
	}

	// 10 answers
	answers := ar.Get(10)

	require.Equal(t, 10, len(answers))

	for i := 0; i < 4; i++ {
		require.Equal(t, CorrectAnswer, answers[i])
	}
	for i := 4; i < 7; i++ {
		require.Equal(t, wrongAnswer, answers[i])
	}
	for i := 7; i < 9; i++ {
		require.Equal(t, byte(3), answers[i])
	}
	require.Equal(t, byte(0), answers[9])

	// 9 answers
	answers = ar.Get(9)

	for i := 0; i < 3; i++ {
		require.Equal(t, CorrectAnswer, answers[i])
	}
	for i := 3; i < 6; i++ {
		require.Equal(t, wrongAnswer, answers[i])
	}
	for i := 6; i < 8; i++ {
		require.Equal(t, byte(3), answers[i])
	}
	require.Equal(t, byte(0), answers[8])

	// 8 answers
	answers = ar.Get(8)

	for i := 0; i < 3; i++ {
		require.Equal(t, CorrectAnswer, answers[i])
	}
	for i := 3; i < 5; i++ {
		require.Equal(t, wrongAnswer, answers[i])
	}
	for i := 5; i < 7; i++ {
		require.Equal(t, byte(3), answers[i])
	}
	require.Equal(t, byte(0), answers[7])
}*/

func TestParseNums(t *testing.T) {
	s := "1"
	users, err := parseNums(s)
	require.Nil(t, err)
	require.Equal(t, 1, len(users))
	require.Equal(t, 1, users[0])

	s = "1,2,34"
	users, err = parseNums(s)
	require.Nil(t, err)
	require.Equal(t, 3, len(users))
	require.Equal(t, 1, users[0])
	require.Equal(t, 2, users[1])
	require.Equal(t, 34, users[2])

	s = "1,2,5-7"
	users, err = parseNums(s)
	require.Nil(t, err)
	require.Equal(t, 5, len(users))
	require.Equal(t, 1, users[0])
	require.Equal(t, 2, users[1])
	require.Equal(t, 5, users[2])
	require.Equal(t, 6, users[3])
	require.Equal(t, 7, users[4])

	s = "1,2,1-3"
	users, err = parseNums(s)
	require.Nil(t, err)
	require.Equal(t, 5, len(users))
	require.Equal(t, 1, users[0])
	require.Equal(t, 2, users[1])
	require.Equal(t, 1, users[2])
	require.Equal(t, 2, users[3])
	require.Equal(t, 3, users[4])

	s = "1,2-3-4"
	users, err = parseNums(s)
	require.NotNil(t, err)
}
