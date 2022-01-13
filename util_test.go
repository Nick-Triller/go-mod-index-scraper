package main

import "testing"

func TestIsPreRelease_TrueForPreRelease(t *testing.T) {
	cases := []struct {
		version string
		actual  bool
	}{
		{version: "v10.10.10-test"},
		{version: "v0.0.0-test"},
		{version: "v3.3.3-test"},
		{version: "v3.3.3-test+asd"},
	}
	for _, c := range cases {
		c.actual = isPreRelease(c.version)
		if c.actual != true {
			t.Errorf("isPreRelease(%s) = %t; want %t", c.version, c.actual, true)
		}
	}
}

func TestIsPreRelease_FalseForRelease(t *testing.T) {
	cases := []struct {
		version string
		actual  bool
	}{
		{version: "v10.10.10"},
		{version: "v10.10.10+asd-test"},
		{version: "v0.0.0"},
		{version: "v0.0.0+asd-test"},
	}
	for _, c := range cases {
		c.actual = isPreRelease(c.version)
		if c.actual != false {
			t.Errorf("isPreRelease(%s) = %t; want %t", c.version, c.actual, false)
		}
	}
}
