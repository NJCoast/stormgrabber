package main

import "testing"

func TestWindfieldTitleExtract(t *testing.T) {
	name, code := ExtractWindTitle("Advisory #033 Wind Field [shp] - Tropical Storm Florence (AT1/AL062018)")
	if name != "florence" {
		t.Errorf("Name was incorrect, got: %s, want: %s.", name, "florence")
	}

    if code != "al062018" {
		t.Errorf("Code was incorrect, got: %s, want: %s.", code, "al062018")
	}

	name, code = ExtractWindTitle("Advisory #001 Wind Field [shp] - Potential Tropical Cyclone Eight (AT3/AL082018)")
	if name != "eight" {
		t.Errorf("Name was incorrect, got: %s, want: %s.", name, "eight")
	}

    if code != "al082018" {
		t.Errorf("Code was incorrect, got: %s, want: %s.", code, "al082018")
	}

	name, code = ExtractWindTitle("Advisory Wind Field [shp] - Tropical Storm ANDREA (AT1/AL012013)")
	if name != "andrea" {
		t.Errorf("Name was incorrect, got: %s, want: %s.", name, "andrea")
	}

    if code != "al012013" {
		t.Errorf("Code was incorrect, got: %s, want: %s.", code, "al012013")
	}
}

func TestTrackTitleExtract(t *testing.T) {
	name, code := ExtractTrackTitle("Preliminary Best Track Points [kmz] - Tropical Storm Florence (AT1/AL062018)")
	if name != "florence" {
		t.Errorf("Name was incorrect, got: %s, want: %s.", name, "florence")
	}

    if code != "al062018" {
		t.Errorf("Code was incorrect, got: %s, want: %s.", code, "al062018")
	}

	name, code = ExtractTrackTitle("Preliminary Best Track Points [kmz] - Potential Tropical Cyclone Eight (AT3/AL082018)")
	if name != "eight" {
		t.Errorf("Name was incorrect, got: %s, want: %s.", name, "eight")
	}

    if code != "al082018" {
		t.Errorf("Code was incorrect, got: %s, want: %s.", code, "al082018")
	}

	name, code = ExtractTrackTitle("Preliminary Best Track Points [kmz] - Tropical Storm ANDREA (AT1/AL012013)")
	if name != "andrea" {
		t.Errorf("Name was incorrect, got: %s, want: %s.", name, "andrea")
	}

    if code != "al012013" {
		t.Errorf("Code was incorrect, got: %s, want: %s.", code, "al012013")
	}
}