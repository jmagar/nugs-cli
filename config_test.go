package main

import (
	"reflect"
	"testing"
)

func TestNormalizeCliAliases(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{name: "list default", in: []string{"list"}, want: []string{"list", "artists"}},
		{name: "list filter short", in: []string{"list", ">100"}, want: []string{"list", "artists", "shows", ">100"}},
		{name: "list venue short", in: []string{"list", "1125", "Red", "Rocks"}, want: []string{"list", "1125", "shows", "Red", "Rocks"}},
		{name: "grab latest", in: []string{"grab", "1125", "latest"}, want: []string{"1125", "latest"}},
		{name: "grab show id", in: []string{"grab", "23329"}, want: []string{"23329"}},
		{name: "grab url", in: []string{"grab", "https://play.nugs.net/release/23329"}, want: []string{"https://play.nugs.net/release/23329"}},
		{name: "grab multiple ids", in: []string{"grab", "23329", "23790"}, want: []string{"23329", "23790"}},
		{name: "catalog short update", in: []string{"update"}, want: []string{"catalog", "update"}},
		{name: "catalog short gaps", in: []string{"gaps", "1125"}, want: []string{"catalog", "gaps", "1125"}},
		{name: "refresh short", in: []string{"refresh", "enable"}, want: []string{"catalog", "config", "enable"}},
		{name: "unchanged old catalog", in: []string{"catalog", "gaps", "1125"}, want: []string{"catalog", "gaps", "1125"}},
		// Media modifier cases
		{name: "list audio modifier", in: []string{"list", "audio"}, want: []string{"list", "artists", "audio"}},
		{name: "list video modifier", in: []string{"list", "video"}, want: []string{"list", "artists", "video"}},
		{name: "list both modifier", in: []string{"list", "both"}, want: []string{"list", "artists", "both"}},
		{name: "list artist video", in: []string{"list", "1125", "video"}, want: []string{"list", "1125", "video"}},
		{name: "list artist audio", in: []string{"list", "1125", "audio"}, want: []string{"list", "1125", "audio"}},
		{name: "list artist venue not media", in: []string{"list", "1125", "Red Rocks"}, want: []string{"list", "1125", "shows", "Red Rocks"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeCliAliases(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("normalizeCliAliases(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
