package acme

import (
	"context"
	"testing"
)

func TestHTTPChallengeProviderLifecycle(t *testing.T) {
	p := newHTTPChallengeProvider()
	if err := p.Present("example.com", "token", "key-auth"); err != nil {
		t.Fatal(err)
	}
	if got, ok := p.Response("token"); !ok || got != "key-auth" {
		t.Fatalf("response = %q, %v", got, ok)
	}
	if err := p.CleanUp("example.com", "token", "key-auth"); err != nil {
		t.Fatal(err)
	}
	if _, ok := p.Response("token"); ok {
		t.Fatal("challenge token remained after cleanup")
	}
}

func TestUpdateEmailBeforeRegistration(t *testing.T) {
	client := &Client{mu: make(chan struct{}, 1), user: &acmeUser{}}
	if err := client.UpdateEmail(context.Background(), " admin@example.com "); err != nil {
		t.Fatal(err)
	}
	if !client.HasEmail() || client.user.email != "admin@example.com" {
		t.Fatalf("email was not updated: client=%q user=%q", client.email, client.user.email)
	}
}
