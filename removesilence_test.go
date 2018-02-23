package main

import (
	"reflect"
	"testing"
)

func TestInvertSegments(t *testing.T) {
	cases := []struct {
		in   []segment
		want []segment
	}{
		{[]segment{}, []segment{{0.0, 0.0}}},
		{[]segment{{1.0, 4.0}}, []segment{{0.0, 1.0}, {4.0, 0.0}}},
		{[]segment{{1.0, 4.0}, {6.5, 9.5}}, []segment{{0.0, 1.0}, {4.0, 6.5}, {9.5, 0.0}}},
	}
	for _, c := range cases {
		got := invertSegments(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("invertSegments(%v)=%v; want %v", c.in, got, c.want)
		}
	}
}

func TestSegmentsToRemove(t *testing.T) {
	cases := []struct {
		silence      []segment
		maxPause     float64
		introPadding float64
		outroPadding float64
		dur          float64
		want         []segment
	}{
		{[]segment{}, 0, 0, 0, 0, []segment{}},
		{[]segment{{0.0, 1.0}}, 0.0, 0, 0, 3.0, []segment{}},
		{[]segment{{0.0, 1.0}}, 0.0, 0.3, 0, 3.0, []segment{{0.0, 0.7}}},
		{[]segment{{0.0, 1.0}}, 0.0, 0.3, 0.3, 3.0, []segment{{0.0, 0.7}}},
		{[]segment{{0.0, 1.0}, {1.0, 3.0}}, 1.0, 0.3, 0.2, 3.0, []segment{{0.0, 0.7}, {1.2, 0}}},
		{[]segment{{0.0, 1.0}, {1.0, 3.0}}, 1.0, 2.0, 0.2, 3.0, []segment{{1.2, 0}}},
		{[]segment{{0.0, 1.0}, {1.0, 3.0}}, 1.0, 0.3, 0.2, 4.0, []segment{{0.0, 0.7}, {1.5, 2.5}}},
	}
	for _, c := range cases {
		got := segmentsToRemove(c.silence, c.maxPause, c.introPadding, c.outroPadding, c.dur)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("segmentsToRemove(%v,%v,%v,%v,%v)=%v; want %v",
				c.silence, c.maxPause, c.introPadding, c.outroPadding, c.dur, got, c.want)
		}
	}
}

func TestFfmpegParseSilentPeriods(t *testing.T) {
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

func TestFfmpegParseDuration(t *testing.T) {
	cases := []struct {
		line string
		want float64
	}{
		{"Duration: 0:0:.1,", 0.1},
		{"Duration: 0:0:1.1,", 1.1},
		{"Duration: 0:1:1.1,", 61.1},
		{"Duration: 1:1:1.1,", 3661.1},
	}
	for _, c := range cases {
		got, err := ffmpegParseDuration([]string{c.line})
		if err != nil {
			t.Errorf("ffmpegParseDuration(%q) returned unexpected error: %s", c.line, err)
		}
		if got != c.want {
			t.Errorf("ffmpegParseDuration(%q)=%v; want %v", c.line, got, c.want)
		}
	}
}
