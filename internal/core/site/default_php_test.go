package site

import (
	"context"
	"testing"
)

type fixedSettings map[string]string

func (settings fixedSettings) Get(_ context.Context, key string) (string, error) {
	return settings[key], nil
}

func TestDefaultPHPIsAppliedOnlyToNewPHPWithoutExplicitVersion(t *testing.T) {
	service := &Service{settings: fixedSettings{"core.default_php": "8.3"}}
	request := CreateRequest{AppType: "php"}
	if err := service.applyDefaultPHP(context.Background(), &request); err != nil {
		t.Fatal(err)
	}
	if request.AppVersion == nil || *request.AppVersion != "8.3" {
		t.Fatalf("version = %#v, want 8.3", request.AppVersion)
	}

	explicit := "8.2"
	request = CreateRequest{AppType: "php", AppVersion: &explicit}
	if err := service.applyDefaultPHP(context.Background(), &request); err != nil {
		t.Fatal(err)
	}
	if request.AppVersion == nil || *request.AppVersion != "8.2" {
		t.Fatalf("explicit version was overwritten: %#v", request.AppVersion)
	}

	request = CreateRequest{AppType: "static"}
	if err := service.applyDefaultPHP(context.Background(), &request); err != nil {
		t.Fatal(err)
	}
	if request.AppVersion != nil {
		t.Fatalf("default PHP leaked into static site: %#v", request.AppVersion)
	}
}
