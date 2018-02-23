package main

import (
	"reflect"
	"testing"
)

func TestInvertSegmentsWithPadding(t *testing.T) {
	cases := []struct {
		padding float64
		in      []segment
		want    []segment
	}{
		{0.0, []segment{}, []segment{{0.0, 0.0}}},
		{1.0, []segment{}, []segment{{0.0, 0.0}}},
		{1.0, []segment{{1.0, 4.0}}, []segment{{0.0, 2.0}, {3.0, 0.0}}},
		{1.0, []segment{{1.0, 4.0}, {6.5, 9.5}}, []segment{{0.0, 2.0}, {3.0, 7.5}, {8.5, 0.0}}},
	}
	for _, c := range cases {
		got := invertSegmentsWithPadding(c.in, c.padding)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("invertSegmentsWithPadding(%v, %f)=%v; want %v", c.in, c.padding, got, c.want)
		}
	}
}

func testFfmpegParseSilentPeriods(t *testing.T) {
	cases := []struct {
		lines []string
		want  []segment
	}{
		{[]string{}, []segment{}},
		{[]string{"a b c silence_start: 1.0", "a b c silence_end: 2.0"}, []segment{{1.0, 2.0}}},
	}
	for _, c := range cases {
		got, err := ffmpegParseSilentPeriods(c.lines)
		if err != nil {
			t.Errorf("ffmpegParseSilentPeriods(%v) returned unexpected error %v", c.lines, err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("ffmpegParseSilentPeriods(%v)=%v; want %v", c.lines, got, c.want)
		}
	}
}
