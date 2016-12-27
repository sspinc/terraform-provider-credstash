package main

import (
	"testing"

	"github.com/hashicorp/terraform/helper/schema"
)

func TestProvider(t *testing.T) {
	if err := provider().(*schema.Provider).InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}
