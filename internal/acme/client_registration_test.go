package acme

import (
	"testing"

	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
)

func TestEnsureRegisteredReusesCachedRegistration(t *testing.T) {
	reg := &registration.Resource{URI: "https://acme.test/account/1"}
	client := &Client{
		legoClient: &lego.Client{},
		user:       &acmeUser{registration: reg},
	}

	if err := client.ensureRegistered(); err != nil {
		t.Fatalf("reuse cached registration: %v", err)
	}
	if client.user.registration != reg {
		t.Fatal("cached registration was replaced")
	}
}
