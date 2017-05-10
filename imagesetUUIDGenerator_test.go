package main

import (
	"testing"
)

func TestGenerateImageSetUUID(t *testing.T) {
	expectedImagesetUUIDString := "f622bfc6-4931-11e4-9d7e-00144feab7de"
	imageUUIDString := "f622bfc6-4931-11e4-0318-978e959e1c97"
	imageUUID, _ := NewUUIDFromString(imageUUIDString)

	imagesetUUID, err := GenerateUUID(*imageUUID)

	if err != nil {
		t.Error("Returned error for valid input")
	}
	actualImagesetUUIDString := imagesetUUID.String()
	if expectedImagesetUUIDString != actualImagesetUUIDString {
		t.Error("Imageset UUID was not generated correctly.")
	}
}

func TestGenerateImageUUIDFromImageSetUUID(t *testing.T) {
	imagesetUUIDString := "f622bfc6-4931-11e4-0318-978e959e1c97"
	expectedImageUUIDString := "f622bfc6-4931-11e4-9d7e-00144feab7de"
	imagesetUUID, _ := NewUUIDFromString(imagesetUUIDString)

	imageUUID, err := GenerateUUID(*imagesetUUID)

	if err != nil {
		t.Error("Returned error for valid input")
	}
	actualImageUUIDString := imageUUID.String()
	if expectedImageUUIDString != actualImageUUIDString {
		t.Error("Image UUID was not generated correctly.")
	}
}
