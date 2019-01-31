package bitmap3_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/funny-falcon/highloadcup2018/bitmap3"
)

func TestAndLikes(t *testing.T) {
	likes1 := &bitmap3.Likes{}
	likes2 := &bitmap3.Likes{}
	likes3 := &bitmap3.Likes{}

	likes1.SetTs(1, 2, 1)
	likes1.SetTs(1, 6, 1)
	likes1.SetTs(1, 9, 1)
	likes1.SetTs(1, 12, 1)
	likes1.SetTs(1, 24, 1)

	likes2.SetTs(1, 1, 1)
	likes2.SetTs(1, 6, 1)
	likes2.SetTs(1, 7, 1)
	likes2.SetTs(1, 10, 1)
	likes2.SetTs(1, 12, 1)

	likes3.SetTs(1, 6, 1)
	likes3.SetTs(1, 9, 1)
	likes3.SetTs(1, 7, 1)
	likes3.SetTs(1, 12, 1)
	likes3.SetTs(1, 15, 1)

	correct := []int32{12, 6}
	require.Equal(t, correct, bitmap3.AndLikes([]*bitmap3.Likes{likes1, likes2, likes3}))
	require.Equal(t, correct, bitmap3.AndLikes([]*bitmap3.Likes{likes1, likes3, likes2}))
	require.Equal(t, correct, bitmap3.AndLikes([]*bitmap3.Likes{likes2, likes1, likes3}))
	require.Equal(t, correct, bitmap3.AndLikes([]*bitmap3.Likes{likes2, likes3, likes1}))
	require.Equal(t, correct, bitmap3.AndLikes([]*bitmap3.Likes{likes3, likes1, likes2}))
	require.Equal(t, correct, bitmap3.AndLikes([]*bitmap3.Likes{likes3, likes2, likes1}))
}
